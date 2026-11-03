package vmselectapi

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricnamestats"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

// testSearchMetricNamesV3API is the fake implementation of the vmselect API
// that is used in TestSearchMetricNamesV3().
type testSearchMetricNamesV3API struct {
	API
	gotSearchQuery *storage.SearchQuery
	metricNames    []string
	err            error
}

func (api *testSearchMetricNamesV3API) Reset(metricNames []string, err string) {
	api.gotSearchQuery = nil
	api.metricNames = metricNames
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testSearchMetricNamesV3API) SearchMetricNames(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery) ([]string, error) {
	api.gotSearchQuery = sq
	return api.metricNames, api.err
}

func TestSearchMetricNamesV3(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	// Start the vmselect RPC server and connect to it.
	api := &testSearchMetricNamesV3API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	// f verifies the RPC protocol of searching metric names.
	f := func(wantMetricNames []string, wantErr string) {
		t.Helper()

		api.Reset(wantMetricNames, wantErr)

		sq := newSearchQuery()

		// Make an RPC, read the resulting error message, and confirm the API on the server side
		// was called the correct arguments.
		c.WriteRPCName("searchMetricNames_v3", false, 1)
		c.WriteSearchQuery(sq)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
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

	// Start the vmselect RPC server and connect to it.
	api := &testSearchMetricNamesV3ClosedConnAPI{
		called:   make(chan struct{}),
		canceled: make(chan struct{}),
	}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)

	sq := newSearchQuery()

	// Make an RPC, wait until the server calls the API, and close the connection.
	// Closing the connection should trigger the API call cancellation.
	c.WriteRPCName("searchMetricNames_v3", false, 1)
	c.WriteSearchQuery(sq)
	<-api.called
	c.Close()
	<-api.canceled
}

// testBlockIterator is the fake implementation of BlockIterator
// that is used in TestSearchV7().
type testBlockIterator struct {
	blocks [][]byte
	closed bool
}

func (bi *testBlockIterator) NextBlock(_ context.Context, dst []byte) ([]byte, bool) {
	if len(bi.blocks) == 0 {
		return dst, false
	}
	dst = append(dst, bi.blocks[0]...)
	bi.blocks = bi.blocks[1:]
	return dst, true
}

func (bi *testBlockIterator) MustClose() {
	bi.closed = true
}

func (bi *testBlockIterator) Error() error {
	return nil
}

// testSearchV7API is the fake implementation of the vmselect API
// that is used in TestSearchV7().
type testSearchV7API struct {
	API
	gotSearchQuery *storage.SearchQuery
	bi             *testBlockIterator
	err            error
}

func (api *testSearchV7API) Reset(blocks []string, err string) {
	api.gotSearchQuery = nil
	api.bi = &testBlockIterator{}
	for _, b := range blocks {
		api.bi.blocks = append(api.bi.blocks, []byte(b))
	}
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testSearchV7API) InitSearch(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery) (BlockIterator, error) {
	api.gotSearchQuery = sq
	if api.err != nil {
		return nil, api.err
	}
	return api.bi, nil
}

func TestSearchV7(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testSearchV7API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantBlocks []string, wantErr string) {
		t.Helper()

		api.Reset(wantBlocks, wantErr)

		sq := newSearchQuery()

		c.WriteRPCName("search_v7", false, 1)
		c.WriteSearchQuery(sq)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		var gotBlocks []string
		for {
			b := c.ReadString()
			if b == "" {
				break
			}
			gotBlocks = append(gotBlocks, b)
		}
		if diff := cmp.Diff(wantBlocks, gotBlocks, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected blocks (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
		if !api.bi.closed {
			t.Fatalf("block iterator must be closed")
		}
	}

	f([]string{}, "")
	f([]string{"block1"}, "")
	f([]string{"block1", "block2"}, "")
	f(nil, "test error")
	f([]string{"block1", "block2", "block3"}, "")
}

// testLabelValuesV5API is the fake implementation of the vmselect API
// that is used in TestLabelValuesV5().
type testLabelValuesV5API struct {
	API
	gotSearchQuery    *storage.SearchQuery
	gotLabelName      string
	gotMaxLabelValues int
	labelValues       []string
	err               error
}

func (api *testLabelValuesV5API) Reset(labelValues []string, err string) {
	api.gotSearchQuery = nil
	api.gotLabelName = ""
	api.gotMaxLabelValues = 0
	api.labelValues = labelValues
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testLabelValuesV5API) LabelValues(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery, labelName string, maxLabelValues int) ([]string, error) {
	api.gotSearchQuery = sq
	api.gotLabelName = labelName
	api.gotMaxLabelValues = maxLabelValues
	return api.labelValues, api.err
}

func TestLabelValuesV5(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testLabelValuesV5API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantLabelValues []string, wantErr string) {
		t.Helper()

		api.Reset(wantLabelValues, wantErr)

		const (
			labelName      = "label1"
			maxLabelValues = 100
		)
		sq := newSearchQuery()

		c.WriteRPCName("labelValues_v5", false, 1)
		c.WriteString(labelName)
		c.WriteSearchQuery(sq)
		c.WriteUint32(maxLabelValues)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}
		if api.gotLabelName != labelName {
			t.Fatalf("unexpected labelName: got %q, want %q", api.gotLabelName, labelName)
		}
		if api.gotMaxLabelValues != maxLabelValues {
			t.Fatalf("unexpected maxLabelValues: got %d, want %d", api.gotMaxLabelValues, maxLabelValues)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		var gotLabelValues []string
		for {
			v := c.ReadString()
			if v == "" {
				break
			}
			gotLabelValues = append(gotLabelValues, v)
		}
		if diff := cmp.Diff(wantLabelValues, gotLabelValues, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected label values (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f([]string{}, "")
	f([]string{"v1"}, "")
	f([]string{"v1", "v2"}, "")
	f(nil, "test error")
	f([]string{"v1", "v2", "v3"}, "")
}

// testTagValueSuffixesV4API is the fake implementation of the vmselect API
// that is used in TestTagValueSuffixesV4().
type testTagValueSuffixesV4API struct {
	API
	gotAccountID      uint32
	gotProjectID      uint32
	gotTimeRange      storage.TimeRange
	gotTagKey         string
	gotTagValuePrefix string
	gotDelimiter      byte
	gotMaxSuffixes    int
	suffixes          []string
	err               error
}

func (api *testTagValueSuffixesV4API) Reset(suffixes []string, err string) {
	api.gotAccountID = 0
	api.gotProjectID = 0
	api.gotTimeRange = storage.TimeRange{}
	api.gotTagKey = ""
	api.gotTagValuePrefix = ""
	api.gotDelimiter = 0
	api.gotMaxSuffixes = 0
	api.suffixes = suffixes
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testTagValueSuffixesV4API) TagValueSuffixes(_ context.Context, _ *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int) ([]string, error) {
	api.gotAccountID = accountID
	api.gotProjectID = projectID
	api.gotTimeRange = tr
	api.gotTagKey = tagKey
	api.gotTagValuePrefix = tagValuePrefix
	api.gotDelimiter = delimiter
	api.gotMaxSuffixes = maxSuffixes
	return api.suffixes, api.err
}

func TestTagValueSuffixesV4(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testTagValueSuffixesV4API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantSuffixes []string, wantErr string) {
		t.Helper()

		api.Reset(wantSuffixes, wantErr)

		const (
			accountID      = 12
			projectID      = 34
			minTimestamp   = 123456789
			maxTimestamp   = 987654321
			tagKey         = "key1"
			tagValuePrefix = "prefix."
			delimiter      = '.'
			maxSuffixes    = 100
		)

		c.WriteRPCName("tagValueSuffixes_v4", false, 1)
		c.WriteUint32(accountID)
		c.WriteUint32(projectID)
		c.WriteUint64(minTimestamp)
		c.WriteUint64(maxTimestamp)
		c.WriteString(tagKey)
		c.WriteString(tagValuePrefix)
		c.WriteUint8(delimiter)
		c.WriteUint32(maxSuffixes)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if api.gotAccountID != accountID {
			t.Fatalf("unexpected accountID: got %d, want %d", api.gotAccountID, accountID)
		}
		if api.gotProjectID != projectID {
			t.Fatalf("unexpected projectID: got %d, want %d", api.gotProjectID, projectID)
		}
		wantTimeRange := storage.TimeRange{
			MinTimestamp: minTimestamp,
			MaxTimestamp: maxTimestamp,
		}
		if api.gotTimeRange != wantTimeRange {
			t.Fatalf("unexpected time range: got %v, want %v", api.gotTimeRange, wantTimeRange)
		}
		if api.gotTagKey != tagKey {
			t.Fatalf("unexpected tagKey: got %q, want %q", api.gotTagKey, tagKey)
		}
		if api.gotTagValuePrefix != tagValuePrefix {
			t.Fatalf("unexpected tagValuePrefix: got %q, want %q", api.gotTagValuePrefix, tagValuePrefix)
		}
		if api.gotDelimiter != delimiter {
			t.Fatalf("unexpected delimiter: got %d, want %d", api.gotDelimiter, delimiter)
		}
		if api.gotMaxSuffixes != maxSuffixes {
			t.Fatalf("unexpected maxSuffixes: got %d, want %d", api.gotMaxSuffixes, maxSuffixes)
		}

		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		n := c.ReadUint64()
		gotSuffixes := make([]string, n)
		for i := range gotSuffixes {
			gotSuffixes[i] = c.ReadString()
		}
		if diff := cmp.Diff(wantSuffixes, gotSuffixes, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected suffixes (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f([]string{}, "")
	f([]string{"s1"}, "")
	f([]string{"", "s1", "s2"}, "")
	f(nil, "test error")
	f([]string{"s1", "s2", "s3"}, "")
}

// testLabelNamesV5API is the fake implementation of the vmselect API
// that is used in TestLabelNamesV5().
type testLabelNamesV5API struct {
	API
	gotSearchQuery   *storage.SearchQuery
	gotMaxLabelNames int
	labelNames       []string
	err              error
}

func (api *testLabelNamesV5API) Reset(labelNames []string, err string) {
	api.gotSearchQuery = nil
	api.gotMaxLabelNames = 0
	api.labelNames = labelNames
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testLabelNamesV5API) LabelNames(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery, maxLabelNames int) ([]string, error) {
	api.gotSearchQuery = sq
	api.gotMaxLabelNames = maxLabelNames
	return api.labelNames, api.err
}

func TestLabelNamesV5(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testLabelNamesV5API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantLabelNames []string, wantErr string) {
		t.Helper()

		api.Reset(wantLabelNames, wantErr)

		const (
			enableQueryTracer = false
			maxLabelNames     = 100
		)
		sq := newSearchQuery()

		c.WriteRPCName("labelNames_v5", false, 1)
		c.WriteSearchQuery(sq)
		c.WriteUint32(maxLabelNames)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}
		if api.gotMaxLabelNames != maxLabelNames {
			t.Fatalf("unexpected maxLabelNames: got %d, want %d", api.gotMaxLabelNames, maxLabelNames)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		var gotLabelNames []string
		for {
			v := c.ReadString()
			if v == "" {
				break
			}
			gotLabelNames = append(gotLabelNames, v)
		}
		if diff := cmp.Diff(wantLabelNames, gotLabelNames, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected label names (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f([]string{}, "")
	f([]string{"l1"}, "")
	f([]string{"l1", "l2"}, "")
	f(nil, "test error")
	f([]string{"l1", "l2", "l3"}, "")
}

// testSeriesCountV4API is the fake implementation of the vmselect API
// that is used in TestSeriesCountV4().
type testSeriesCountV4API struct {
	API
	gotAccountID uint32
	gotProjectID uint32
	n            uint64
	err          error
}

func (api *testSeriesCountV4API) Reset(n uint64, err string) {
	api.gotAccountID = 0
	api.gotProjectID = 0
	api.n = n
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testSeriesCountV4API) SeriesCount(_ context.Context, _ *querytracer.Tracer, accountID, projectID uint32) (uint64, error) {
	api.gotAccountID = accountID
	api.gotProjectID = projectID
	return api.n, api.err
}

func TestSeriesCountV4(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testSeriesCountV4API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantN uint64, wantErr string) {
		t.Helper()

		api.Reset(wantN, wantErr)

		const (
			accountID = 12
			projectID = 34
		)

		c.WriteRPCName("seriesCount_v4", false, 1)
		c.WriteUint32(accountID)
		c.WriteUint32(projectID)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if api.gotAccountID != accountID || api.gotProjectID != projectID {
			t.Fatalf("unexpected tenant: got %d:%d, want %d:%d", api.gotAccountID, api.gotProjectID, accountID, projectID)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		if got := c.ReadUint64(); got != wantN {
			t.Fatalf("unexpected series count: got %d, want %d", got, wantN)
		}
		c.ReadEmptyTrace()
	}

	f(0, "")
	f(123, "")
	f(0, "test error")
	f(456, "")
}

// testTSDBStatusV6API is the fake implementation of the vmselect API
// that is used in TestTSDBStatusV6().
type testTSDBStatusV6API struct {
	API
	gotSearchQuery *storage.SearchQuery
	gotFocusLabel  string
	gotTopN        int
	status         *storage.TSDBStatus
	err            error
}

func (api *testTSDBStatusV6API) Reset(status *storage.TSDBStatus, err string) {
	api.gotSearchQuery = nil
	api.gotFocusLabel = ""
	api.gotTopN = 0
	api.status = status
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testTSDBStatusV6API) TSDBStatus(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery, focusLabel string, topN int) (*storage.TSDBStatus, error) {
	api.gotSearchQuery = sq
	api.gotFocusLabel = focusLabel
	api.gotTopN = topN
	return api.status, api.err
}

func TestTSDBStatusV6(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testTSDBStatusV6API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	readTopHeapEntries := func() []storage.TopHeapEntry {
		t.Helper()
		entries := make([]storage.TopHeapEntry, c.ReadUint64())
		for i := range entries {
			entries[i].Name = c.ReadString()
			entries[i].Count = c.ReadUint64()
		}
		return entries
	}

	f := func(wantStatus *storage.TSDBStatus, wantErr string) {
		t.Helper()

		api.Reset(wantStatus, wantErr)

		const (
			focusLabel = "label1"
			topN       = 10
		)
		sq := newSearchQuery()

		c.WriteRPCName("tsdbStatus_v6", false, 1)
		c.WriteSearchQuery(sq)
		c.WriteString(focusLabel)
		c.WriteUint32(topN)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}
		if api.gotFocusLabel != focusLabel {
			t.Fatalf("unexpected focusLabel: got %q, want %q", api.gotFocusLabel, focusLabel)
		}
		if api.gotTopN != topN {
			t.Fatalf("unexpected topN: got %d, want %d", api.gotTopN, topN)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		gotStatus := &storage.TSDBStatus{
			TotalSeries:                  c.ReadUint64(),
			TotalLabelValuePairs:         c.ReadUint64(),
			SeriesCountByMetricName:      readTopHeapEntries(),
			SeriesCountByLabelName:       readTopHeapEntries(),
			SeriesCountByFocusLabelValue: readTopHeapEntries(),
			SeriesCountByLabelValuePair:  readTopHeapEntries(),
			LabelValueCountByLabelName:   readTopHeapEntries(),
		}
		records := make([]metricnamestats.StatRecord, c.ReadUint64())
		for i := range records {
			records[i].MetricName = c.ReadString()
			records[i].LastRequestTs = c.ReadUint64()
			records[i].RequestsCount = c.ReadUint64()
		}
		gotStatus.SeriesQueryStatsByMetricName = records
		if diff := cmp.Diff(wantStatus, gotStatus, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected status (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f(&storage.TSDBStatus{}, "")
	f(&storage.TSDBStatus{
		TotalSeries:                  100,
		TotalLabelValuePairs:         200,
		SeriesCountByMetricName:      []storage.TopHeapEntry{{Name: "m1", Count: 1}, {Name: "m2", Count: 2}},
		SeriesCountByLabelName:       []storage.TopHeapEntry{{Name: "l1", Count: 3}},
		SeriesCountByFocusLabelValue: []storage.TopHeapEntry{{Name: "v1", Count: 4}},
		SeriesCountByLabelValuePair:  []storage.TopHeapEntry{{Name: "l1=v1", Count: 5}},
		LabelValueCountByLabelName:   []storage.TopHeapEntry{{Name: "l1", Count: 6}},
		SeriesQueryStatsByMetricName: []metricnamestats.StatRecord{{MetricName: "m1", RequestsCount: 7, LastRequestTs: 8}},
	}, "")
	f(nil, "test error")
	f(&storage.TSDBStatus{TotalSeries: 1}, "")
}

// testDeleteSeriesV5API is the fake implementation of the vmselect API
// that is used in TestDeleteSeriesV5().
type testDeleteSeriesV5API struct {
	API
	gotSearchQuery *storage.SearchQuery
	deletedCount   int
	err            error
}

func (api *testDeleteSeriesV5API) Reset(deletedCount int, err string) {
	api.gotSearchQuery = nil
	api.deletedCount = deletedCount
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testDeleteSeriesV5API) DeleteSeries(_ context.Context, _ *querytracer.Tracer, sq *storage.SearchQuery) (int, error) {
	api.gotSearchQuery = sq
	return api.deletedCount, api.err
}

func TestDeleteSeriesV5(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testDeleteSeriesV5API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantDeletedCount int, wantErr string) {
		t.Helper()

		api.Reset(wantDeletedCount, wantErr)

		sq := newSearchQuery()

		c.WriteRPCName("deleteSeries_v5", false, 1)
		c.WriteSearchQuery(sq)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(sq, api.gotSearchQuery); diff != "" {
			t.Fatalf("unexpected search query (-want, +got):\n%s", diff)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		if got, want := c.ReadUint64(), uint64(wantDeletedCount); got != want {
			t.Fatalf("unexpected deleted count: got %d, want %d", got, want)
		}
		c.ReadEmptyTrace()
	}

	f(0, "")
	f(123, "")
	f(0, "test error")
	f(456, "")
}

// testRegisterMetricNamesV3API is the fake implementation of the vmselect API
// that is used in TestRegisterMetricNamesV3().
type testRegisterMetricNamesV3API struct {
	API
	gotMetricRows []storage.MetricRow
	err           error
}

func (api *testRegisterMetricNamesV3API) Reset(err string) {
	api.gotMetricRows = nil
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testRegisterMetricNamesV3API) RegisterMetricNames(_ context.Context, _ *querytracer.Tracer, mrs []storage.MetricRow) error {
	api.gotMetricRows = mrs
	return api.err
}

func TestRegisterMetricNamesV3(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testRegisterMetricNamesV3API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(mrs []storage.MetricRow, wantErr string) {
		t.Helper()

		api.Reset(wantErr)

		c.WriteRPCName("registerMetricNames_v3", false, 1)
		c.WriteUint64(uint64(len(mrs)))
		for _, mr := range mrs {
			c.WriteBytes(mr.MetricNameRaw)
			c.WriteUint64(uint64(mr.Timestamp))
		}
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(mrs, api.gotMetricRows, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected metric rows (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f([]storage.MetricRow{}, "")
	f([]storage.MetricRow{{MetricNameRaw: []byte("m1"), Timestamp: 123}}, "")
	f([]storage.MetricRow{{MetricNameRaw: []byte("m1"), Timestamp: 123}}, "test error")
	f([]storage.MetricRow{
		{MetricNameRaw: []byte("m1"), Timestamp: 123},
		{MetricNameRaw: []byte("m2"), Timestamp: 456},
	}, "")
}

// testTenantsV1API is the fake implementation of the vmselect API
// that is used in TestTenantsV1().
type testTenantsV1API struct {
	API
	gotTimeRange storage.TimeRange
	tenants      []string
	err          error
}

func (api *testTenantsV1API) Reset(tenants []string, err string) {
	api.gotTimeRange = storage.TimeRange{}
	api.tenants = tenants
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testTenantsV1API) Tenants(_ context.Context, _ *querytracer.Tracer, tr storage.TimeRange) ([]string, error) {
	api.gotTimeRange = tr
	return api.tenants, api.err
}

func TestTenantsV1(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testTenantsV1API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(wantTenants []string, wantErr string) {
		t.Helper()

		api.Reset(wantTenants, wantErr)

		const (
			minTimestamp = 123456789
			maxTimestamp = 987654321
		)

		c.WriteRPCName("tenants_v1", false, 1)
		c.WriteUint64(minTimestamp)
		c.WriteUint64(maxTimestamp)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		wantTimeRange := storage.TimeRange{
			MinTimestamp: minTimestamp,
			MaxTimestamp: maxTimestamp,
		}
		if api.gotTimeRange != wantTimeRange {
			t.Fatalf("unexpected time range: got %v, want %v", api.gotTimeRange, wantTimeRange)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		var gotTenants []string
		for {
			v := c.ReadString()
			if v == "" {
				break
			}
			gotTenants = append(gotTenants, v)
		}
		if diff := cmp.Diff(wantTenants, gotTenants, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected tenants (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	f([]string{}, "")
	f([]string{"1:2"}, "")
	f([]string{"1:2", "3:4"}, "")
	f(nil, "test error")
	f([]string{"1:2", "3:4", "5:6"}, "")
}

// testMetricNamesUsageStatsV1API is the fake implementation of the vmselect API
// that is used in TestMetricNamesUsageStatsV1().
type testMetricNamesUsageStatsV1API struct {
	API
	gotTenantToken  *storage.TenantToken
	gotLimit        int
	gotLE           int
	gotMatchPattern string
	result          metricnamestats.StatsResult
	err             error
}

func (api *testMetricNamesUsageStatsV1API) Reset(result metricnamestats.StatsResult, err string) {
	api.gotTenantToken = nil
	api.gotLimit = 0
	api.gotLE = 0
	api.gotMatchPattern = ""
	api.result = result
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testMetricNamesUsageStatsV1API) GetMetricNamesUsageStats(_ context.Context, _ *querytracer.Tracer, tt *storage.TenantToken, limit, le int, matchPattern string) (metricnamestats.StatsResult, error) {
	api.gotTenantToken = tt
	api.gotLimit = limit
	api.gotLE = le
	api.gotMatchPattern = matchPattern
	return api.result, api.err
}

func TestMetricNamesUsageStatsV1(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testMetricNamesUsageStatsV1API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(tt *storage.TenantToken, wantResult metricnamestats.StatsResult, wantErr string) {
		t.Helper()

		api.Reset(wantResult, wantErr)

		const (
			limit        = 100
			le           = 5
			matchPattern = "m*"
		)

		c.WriteRPCName("metricNamesUsageStats_v1", false, 1)
		c.WriteBool(tt != nil)
		if tt != nil {
			c.WriteUint32(tt.AccountID)
			c.WriteUint32(tt.ProjectID)
		}
		c.WriteLimit(limit)
		c.WriteInt64(le)
		c.WriteString(matchPattern)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(tt, api.gotTenantToken); diff != "" {
			t.Fatalf("unexpected tenant token (-want, +got):\n%s", diff)
		}
		if api.gotLimit != limit || api.gotLE != le || api.gotMatchPattern != matchPattern {
			t.Fatalf("unexpected args: got limit=%d, le=%d, matchPattern=%q; want limit=%d, le=%d, matchPattern=%q",
				api.gotLimit, api.gotLE, api.gotMatchPattern, limit, le, matchPattern)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		gotResult := metricnamestats.StatsResult{
			CollectedSinceTs: c.ReadUint64(),
			TotalRecords:     c.ReadUint64(),
			CurrentSizeBytes: c.ReadUint64(),
			MaxSizeBytes:     c.ReadUint64(),
			Records:          make([]metricnamestats.StatRecord, c.ReadUint64()),
		}
		for i := range gotResult.Records {
			r := &gotResult.Records[i]
			r.MetricName = c.ReadString()
			r.LastRequestTs = c.ReadUint64()
			r.RequestsCount = c.ReadUint64()
		}
		if diff := cmp.Diff(wantResult, gotResult, cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("unexpected result (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	result := metricnamestats.StatsResult{
		CollectedSinceTs: 1,
		TotalRecords:     2,
		MaxSizeBytes:     3,
		CurrentSizeBytes: 4,
		Records: []metricnamestats.StatRecord{
			{MetricName: "m1", RequestsCount: 5, LastRequestTs: 6},
			{MetricName: "m2", RequestsCount: 7, LastRequestTs: 8},
		},
	}
	f(nil, metricnamestats.StatsResult{}, "")
	f(nil, result, "")
	f(&storage.TenantToken{AccountID: 12, ProjectID: 34}, result, "")
	f(nil, metricnamestats.StatsResult{}, "test error")
	f(&storage.TenantToken{AccountID: 12, ProjectID: 34}, result, "")
}

// testResetMetricNamesStatsV1API is the fake implementation of the vmselect API
// that is used in TestResetMetricNamesStatsV1().
type testResetMetricNamesStatsV1API struct {
	API
	calls int
}

func (api *testResetMetricNamesStatsV1API) ResetMetricNamesUsageStats(_ context.Context, _ *querytracer.Tracer) error {
	api.calls++
	return nil
}

func TestResetMetricNamesStatsV1(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testResetMetricNamesStatsV1API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	for i := range 3 {
		c.WriteRPCName("resetMetricNamesStats_v1", false, 1)
		c.ReadEmptyTrace()
		if got, want := api.calls, i+1; got != want {
			t.Fatalf("unexpected number of API calls: got %d, want %d", got, want)
		}
	}
}

// testSearchMetadataV1API is the fake implementation of the vmselect API
// that is used in TestSearchMetadataV1().
type testSearchMetadataV1API struct {
	API
	gotTenantToken *storage.TenantToken
	gotLimit       int
	gotMetricName  string
	rows           []*metricsmetadata.Row
	err            error
}

func (api *testSearchMetadataV1API) Reset(rows []*metricsmetadata.Row, err string) {
	api.gotTenantToken = nil
	api.gotLimit = 0
	api.gotMetricName = ""
	api.rows = rows
	api.err = nil
	if err != "" {
		api.err = errors.New(err)
	}
}

func (api *testSearchMetadataV1API) GetMetadataRecords(_ context.Context, _ *querytracer.Tracer, tt *storage.TenantToken, limit int, metricName string) ([]*metricsmetadata.Row, error) {
	api.gotTenantToken = tt
	api.gotLimit = limit
	api.gotMetricName = metricName
	return api.rows, api.err
}

func TestSearchMetadataV1(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	api := &testSearchMetadataV1API{}
	s := startServer(t, api)
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	f := func(tt *storage.TenantToken, wantRows []*metricsmetadata.Row, wantErr string) {
		t.Helper()

		api.Reset(wantRows, wantErr)

		const (
			limit      = 100
			metricName = "metric1"
		)

		c.WriteRPCName("searchMetadata_v1", false, 1)
		c.WriteBool(tt != nil)
		if tt != nil {
			c.WriteUint32(tt.AccountID)
			c.WriteUint32(tt.ProjectID)
		}
		c.WriteLimit(limit)
		c.WriteString(metricName)
		if gotErr := c.ReadString(); gotErr != wantErr {
			t.Fatalf("unexpected remote error: got %q, want %q", gotErr, wantErr)
		}
		if diff := cmp.Diff(tt, api.gotTenantToken); diff != "" {
			t.Fatalf("unexpected tenant token (-want, +got):\n%s", diff)
		}
		if api.gotLimit != limit || api.gotMetricName != metricName {
			t.Fatalf("unexpected args: got limit=%d, metricName=%q; want limit=%d, metricName=%q",
				api.gotLimit, api.gotMetricName, limit, metricName)
		}
		if wantErr != "" {
			c.ReadEmptyTrace()
			return
		}

		gotRows := make([]*metricsmetadata.Row, c.ReadUint64())
		for i := range gotRows {
			gotRows[i] = &metricsmetadata.Row{}
			tail, err := gotRows[i].Unmarshal(c.ReadBytes())
			if err != nil {
				t.Fatalf("cannot unmarshal metadata row: %s", err)
			}
			if len(tail) > 0 {
				t.Fatalf("unexpected non-empty tail after unmarshaling metadata row: %q", tail)
			}
		}
		if diff := cmp.Diff(wantRows, gotRows, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(metricsmetadata.Row{})); diff != "" {
			t.Fatalf("unexpected rows (-want, +got):\n%s", diff)
		}
		c.ReadEmptyTrace()
	}

	rows := []*metricsmetadata.Row{
		{
			MetricFamilyName: []byte("metric1"),
			Help:             []byte("help1"),
			Unit:             []byte("seconds"),
			AccountID:        12,
			ProjectID:        34,
			Type:             prompb.MetricTypeCounter,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Help:             []byte("help2"),
			Unit:             []byte("bytes"),
			Type:             prompb.MetricTypeGauge,
		},
	}
	f(nil, []*metricsmetadata.Row{}, "")
	f(nil, rows, "")
	f(&storage.TenantToken{AccountID: 12, ProjectID: 34}, rows[:1], "")
	f(nil, nil, "test error")
	f(&storage.TenantToken{AccountID: 12, ProjectID: 34}, rows, "")
}

// testUnsupportedRPCAPI is a fake implementation of API used in
// TestUnsupportedRPC().
type testUnsupportedRPCAPI struct {
	API
}

func TestUnsupportedRPC(t *testing.T) {
	defer metrics.UnregisterAllMetrics()

	s := startServer(t, &testUnsupportedRPCAPI{})
	defer s.MustStop()
	c := newConn(t, s)
	defer c.Close()

	// The server closes the connection on unsupported rpcName.
	c.WriteRPCName("unsupported_v1", false, 1)
	if _, err := io.ReadFull(c.bc, make([]byte, 1)); err == nil {
		t.Fatalf("expecting non-nil error when reading from conn closed by server")
	}
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

func (c *conn) WriteRPCName(name string, enableQueryTracer bool, timeoutSecs uint32) {
	c.t.Helper()
	c.WriteString(name)
	c.WriteBool(enableQueryTracer)
	c.WriteUint32(timeoutSecs)
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

func (c *conn) WriteLimit(limit int) {
	c.t.Helper()
	if limit < 0 {
		limit = 0
	}
	if limit > 1<<31-1 {
		limit = 1<<31 - 1
	}
	limitU32 := uint32(limit)
	c.WriteUint32(limitU32)
}

func (c *conn) WriteInt64(n int64) {
	c.t.Helper()
	sizeBuf := encoding.MarshalInt64(nil, n)
	if _, err := c.bc.Write(sizeBuf); err != nil {
		c.t.Fatalf("could not write int64 %d to conn: %s", n, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush int64: %s", err)
	}
}

func (c *conn) WriteUint64(n uint64) {
	c.t.Helper()
	buf := encoding.MarshalUint64(nil, n)
	if _, err := c.bc.Write(buf); err != nil {
		c.t.Fatalf("could not write uint64 %d to conn: %s", n, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush uint64: %s", err)
	}
}

func (c *conn) WriteUint8(b byte) {
	c.t.Helper()
	if _, err := c.bc.Write([]byte{b}); err != nil {
		c.t.Fatalf("could not write byte %d to conn: %s", b, err)
	}
	if err := c.bc.Flush(); err != nil {
		c.t.Fatalf("could not flush byte: %s", err)
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

// ReadEmptyTrace reads the query trace from conn and verifies it is empty,
// since all tests disable query tracing.
func (c *conn) ReadEmptyTrace() {
	c.t.Helper()
	if got := c.ReadString(); got != "" {
		c.t.Fatalf("unexpected trace: got %q, want empty", got)
	}
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

func newSearchQuery() *storage.SearchQuery {
	const (
		start      = 123456789
		end        = 987654321
		maxMetrics = 1000000
		accountID  = 12
		projectID  = 34
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

	sq := storage.NewSearchQuery(start, end, tfss, maxMetrics)
	sq.TenantTokens = []storage.TenantToken{
		{
			AccountID: accountID,
			ProjectID: projectID,
		},
	}
	sq.AccountID = accountID
	sq.ProjectID = projectID
	return sq
}

func tf(key, value []byte, isNegative, isRegexp bool) storage.TagFilter {
	return storage.TagFilter{
		Key:        key,
		Value:      value,
		IsNegative: isNegative,
		IsRegexp:   isRegexp,
	}
}
