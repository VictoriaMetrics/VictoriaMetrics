package vmselectapi

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/google/go-cmp/cmp"
)

// testSearchMetricNamesV3API is the fake implementation of the vmselect API
// that is used in TestSearchMetricNamesV3().
type testSearchMetricNamesV3API struct {
	API
	gotSearchQuery *storage.SearchQuery
	metricNames    []string
	err            error
}

func (api *testSearchMetricNamesV3API) SearchMetricNames(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery) ([]string, error) {
	api.gotSearchQuery = sq
	return api.metricNames, api.err
}

func TestSearchMetricNamesV3(t *testing.T) {
	defer metrics.UnregisterAllMetrics()
	const (
		enableQueryTracer = false
		timeoutSecs       = 1
		accountID         = 12
		projectID         = 34
		start             = 123456789
		end               = 987654321
		maxMetrics        = 1000000
		maxBytesSize      = 64 * 1024
	)
	tfss := [][]storage.TagFilter{
		{
			tf(nil, []byte("metric1"), false, false),
			tf([]byte("label2"), []byte("value2"), true, false),
		},
		{
			tf(nil, []byte("metric3"), false, true),
			tf([]byte("label4"), []byte("value4"), true, true),
		},
	}
	sq := newSearchQuery(accountID, projectID, start, end, tfss, maxMetrics)

	// Start the vmselect RPC server and connect to it.
	api := &testSearchMetricNamesV3API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	// f verifies the RPC protocol of searching metric names.
	f := func(wantMetricNames []string, wantErr string) {
		t.Helper()

		// Reset the API.
		api.gotSearchQuery = nil
		api.metricNames = wantMetricNames
		api.err = nil
		if wantErr != "" {
			api.err = errors.New(wantErr)
		}

		// Make an RPC, read the resulting error message, and confirm the API on the server side
		// was called the correct arguments.
		c.WriteString("searchMetricNames_v3")
		c.WriteBool(enableQueryTracer)
		c.WriteUint32(timeoutSecs)
		c.WriteSearchQuery(sq)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		wantSearchQuery := newSearchQuery(accountID, projectID, start, end, tfss, maxMetrics)
		wantSearchQuery.AccountID = accountID
		wantSearchQuery.ProjectID = projectID
		if diff := cmp.Diff(wantSearchQuery, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}

		if wantErr != "" {
			// Non empty error message read from the conn means that
			// API.SearchMetricNames() returned an error. Thus, just read the
			// query trace from conn and return.
			want := ""
			if got := c.ReadString(); got != want {
				t.Fatalf("unexpected trace: got: %s, want %s", got, want)
			}
			return
		}

		// If API.SearchMetricNames() returned no error, then read 0 or more
		// metric names and the query trace from the conn.
		wantNumMetricNames := len(wantMetricNames)
		if got, want := c.ReadUint64(), uint64(wantNumMetricNames); got != want {
			t.Fatalf("unexpected number of metric names: got %d, want %d", got, want)
		}
		gotMetricNames := make([]string, wantNumMetricNames)
		for i := range wantNumMetricNames {
			gotMetricNames[i] = c.ReadString()
		}
		if diff := cmp.Diff(wantMetricNames, gotMetricNames); diff != "" {
			t.Fatalf("unexpected metricNames (-want, +got):\n%s", diff)
		}
		if got, want := c.ReadString(), ""; got != want {
			t.Fatalf("unexpected trace: got: %s, want %s", got, want)
		}
	}

	// Confirm several RPC can be executed on a single connection including
	// cases when the RPC returns an error.
	f([]string{}, "")
	f([]string{"m1"}, "")
	f([]string{"m1", "m2"}, "")
	f(nil, "test error")
	f([]string{"m1", "m2", "m3"}, "")
}

// testSearchMetricNamesV3ClosedConnAPI is the fake implementation of the
// vmselect API that is used in TestSearchMetricNamesV3ClosedConn().
type testSearchMetricNamesV3ClosedConnAPI struct {
	API
	called   chan struct{}
	canceled chan struct{}
}

func (api *testSearchMetricNamesV3ClosedConnAPI) SearchMetricNames(ctx context.Context, _ *querytracer.Tracer, _ *storage.SearchQuery) ([]string, error) {
	close(api.called)

	tt := time.NewTimer(time.Second)
	defer tt.Stop()
	select {
	case <-ctx.Done():
		close(api.canceled)
	case <-tt.C:
		panic("timed out while waiting for API call to be canceled")
	}
	return nil, ctx.Err()
}

func TestSearchMetricNamesV3ClosedConn(t *testing.T) {
	defer metrics.UnregisterAllMetrics()
	const (
		enableQueryTracer = false
		timeoutSecs       = 1
	)
	tfss := [][]storage.TagFilter{{tf(nil, []byte("metric1"), false, false)}}
	sq := newSearchQuery(12, 34, 56, 78, tfss, 90)

	// Start the vmselect RPC server and connect to it.
	api := &testSearchMetricNamesV3ClosedConnAPI{
		called:   make(chan struct{}),
		canceled: make(chan struct{}),
	}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)

	// Make an RPC, wait until the server calls the API, and close the connection.
	// was called the correct arguments. Closing the connection should trigger
	// the API call cancelation.
	c.WriteString("searchMetricNames_v3")
	c.WriteBool(enableQueryTracer)
	c.WriteUint32(timeoutSecs)
	c.WriteSearchQuery(sq)
	<-api.called
	c.Close()
	<-api.canceled
}

func startServer(t *testing.T, api API) *Server {
	t.Helper()

	limits := Limits{
		MaxConcurrentRequests:         1,
		MaxConcurrentRequestsFlagName: "unused",
		MaxQueueDuration:              time.Second,
		MaxQueueDurationFlagName:      "unused",
	}
	disableRPCCompression := true
	s, err := NewServer("localhost:0", api, limits, disableRPCCompression)
	if err != nil {
		t.Fatalf("cannot create a server: %s", err)
	}

	return s
}

type conn struct {
	t  *testing.T
	bc *handshake.BufferedConn
}

func newConn(t *testing.T, s *Server) *conn {
	t.Helper()
	serverAddr := s.ln.Addr().String()
	c, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("could not connect to server at %q: %s", serverAddr, err)
	}
	bc, err := handshake.VMSelectClient(c, 0)
	if err != nil {
		t.Fatalf("could not setup vmselect connection: %s", err)
	}
	return &conn{t, bc}
}

func (c *conn) Close() {
	c.t.Helper()
	if err := c.bc.Close(); err != nil {
		c.t.Fatalf("could not close conn: %s", err)
	}
}

func (c *conn) WriteString(s string) {
	c.t.Helper()
	c.WriteBytes([]byte(s))
}

func (c *conn) WriteBytes(buf []byte) {
	c.t.Helper()
	size := encoding.MarshalUint64(nil, uint64(len(buf)))
	if _, err := c.bc.Write(size); err != nil {
		c.t.Fatalf("could not write bytes size '%d' to conn: %s", size, err)
	}
	if _, err := c.bc.Write(buf); err != nil {
		c.t.Fatalf("could not write bytes %v to conn: %s", buf, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush bytes: %s", err)
	}

}

func (c *conn) WriteBool(b bool) {
	c.t.Helper()
	buf := []byte{0}
	if b {
		buf[0] = 1
	}
	if _, err := c.bc.Write(buf); err != nil {
		c.t.Fatalf("could not write bool %t to conn: %s", b, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush bool: %s", err)
	}
}

func (c *conn) WriteUint32(n uint32) {
	c.t.Helper()
	buf := encoding.MarshalUint32(nil, n)
	if _, err := c.bc.Write(buf); err != nil {
		c.t.Fatalf("could not write uint32 %d to conn: %s", n, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush uint32: %s", err)
	}
}

func (c *conn) WriteSearchQuery(sq *storage.SearchQuery) {
	c.t.Helper()
	var buf []byte
	buf = sq.TenantTokens[0].Marshal(buf)
	buf = sq.MarshalWithoutTenant(buf)
	c.WriteBytes(buf)
}

func (c *conn) ReadString() string {
	c.t.Helper()
	return string(c.ReadBytes())
}

func (c *conn) ReadBytes() []byte {
	c.t.Helper()
	dataSizeBytes := make([]byte, 8)
	if _, err := io.ReadFull(c.bc, dataSizeBytes); err != nil {
		c.t.Fatalf("could not read bytes size: %s", err)
	}
	dataSize := encoding.UnmarshalUint64(dataSizeBytes)
	const maxDataSize = 64 * 1024
	if dataSize > maxDataSize {
		c.t.Fatalf("too big data size: got %d, want <= %d", dataSize, maxDataSize)
	}

	data := make([]byte, dataSize)
	if dataSize == 0 {
		return data
	}
	if _, err := io.ReadFull(c.bc, data); err != nil {
		c.t.Fatalf("cannot read bytes: %s", err)
	}
	return data
}

func (c *conn) ReadUint64() uint64 {
	c.t.Helper()
	buf := make([]byte, 8)
	if _, err := io.ReadFull(c.bc, buf); err != nil {
		c.t.Fatalf("could not read uint64: %s", err)
	}
	return encoding.UnmarshalUint64(buf)
}

func tf(key, value []byte, isNegative, isRegexp bool) storage.TagFilter {
	return storage.TagFilter{
		Key:        key,
		Value:      value,
		IsNegative: isNegative,
		IsRegexp:   isRegexp,
	}
}

func newSearchQuery(accountID, projectID uint32, start, end int64, tfss [][]storage.TagFilter, maxMetrics int) *storage.SearchQuery {
	sq := storage.NewSearchQuery(start, end, tfss, maxMetrics)
	sq.TenantTokens = []storage.TenantToken{
		{
			AccountID: accountID,
			ProjectID: projectID,
		},
	}
	return sq
}
