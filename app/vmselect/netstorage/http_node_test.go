package netstorage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func testHTTPDeadline() searchutil.Deadline {
	return searchutil.DeadlineFromTimestamp(fasttime.UnixTimestamp() + 60)
}

func testHTTPNode(ts *httptest.Server) *httpSelectNode {
	return newHTTPSelectNode(metrics.NewSet(), ts.URL)
}

func testQT() *querytracer.Tracer {
	return querytracer.New(false, "test")
}

// makeSearchQueryData creates a binary-encoded SearchQuery for use in tests.
func makeSearchQueryData(accountID, projectID uint32) []byte {
	sq := storage.NewSearchQuery(accountID, projectID, 1000, 2000, nil, 0)
	return marshalSearchQueryData(sq)
}

// jsonStringsResponse is the JSON format returned by string-list endpoints.
type jsonStringsResponse struct {
	Data      []string `json:"data"`
	IsPartial bool     `json:"isPartial"`
}

func writeStringsResponse(w http.ResponseWriter, data []string, isPartial bool) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonStringsResponse{Data: data, IsPartial: isPartial}) //nolint:errcheck
}

// TestHTTPSelectNodeAddr verifies that addr() returns the base URL.
func TestHTTPSelectNodeAddr(t *testing.T) {
	ms := metrics.NewSet()
	sn := newHTTPSelectNode(ms, "vmselect:8481")
	if sn.addr() != "vmselect:8481" {
		t.Fatalf("unexpected addr: %q", sn.addr())
	}
}

// TestHTTPSelectNodeGetLabelNames verifies that getLabelNames encodes the body correctly
// and decodes the JSON response into (labels, isPartial).
func TestHTTPSelectNodeGetLabelNames(t *testing.T) {
	wantLabels := []string{"__name__", "instance", "job"}
	wantPartial := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"labelNames" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// Last 4 bytes = uint32(maxLabelNames)
		if len(body) < 4 {
			t.Errorf("body too short: %d", len(body))
			http.Error(w, "too short", http.StatusBadRequest)
			return
		}
		maxLabelNames := int(encoding.UnmarshalUint32(body[len(body)-4:]))
		if maxLabelNames != 100 {
			t.Errorf("expected maxLabelNames=100, got %d", maxLabelNames)
		}
		writeStringsResponse(w, wantLabels, wantPartial)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	labels, isPartial, err := sn.getLabelNames(testQT(), makeSearchQueryData(1, 2), 100, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial != wantPartial {
		t.Errorf("isPartial: got %v, want %v", isPartial, wantPartial)
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("labels: got %v, want %v", labels, wantLabels)
	}
}

// TestHTTPSelectNodeGetLabelNamesError verifies that a non-200 response returns an error.
func TestHTTPSelectNodeGetLabelNamesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	_, _, err := sn.getLabelNames(testQT(), makeSearchQueryData(1, 2), 10, testHTTPDeadline())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestHTTPSelectNodeGetLabelValues verifies body encoding for getLabelValues.
func TestHTTPSelectNodeGetLabelValues(t *testing.T) {
	wantValues := []string{"host1", "host2"}
	wantPartial := false

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"labelValues" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// First 8 bytes = uint64(len(labelName))
		if len(body) < 8 {
			t.Errorf("body too short: %d", len(body))
			http.Error(w, "too short", http.StatusBadRequest)
			return
		}
		labelNameLen := int(encoding.UnmarshalUint64(body[:8]))
		body = body[8:]
		if len(body) < labelNameLen {
			t.Errorf("label name truncated")
			http.Error(w, "truncated", http.StatusBadRequest)
			return
		}
		labelName := string(body[:labelNameLen])
		if labelName != "instance" {
			t.Errorf("expected labelName=%q, got %q", "instance", labelName)
		}
		// Last 4 bytes = uint32(maxLabelValues)
		body = body[labelNameLen:]
		maxLabelValues := int(encoding.UnmarshalUint32(body[len(body)-4:]))
		if maxLabelValues != 50 {
			t.Errorf("expected maxLabelValues=50, got %d", maxLabelValues)
		}
		writeStringsResponse(w, wantValues, wantPartial)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	values, isPartial, err := sn.getLabelValues(testQT(), "instance", makeSearchQueryData(1, 2), 50, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial != wantPartial {
		t.Errorf("isPartial: got %v, want %v", isPartial, wantPartial)
	}
	if !reflect.DeepEqual(values, wantValues) {
		t.Errorf("values: got %v, want %v", values, wantValues)
	}
}

// TestHTTPSelectNodeGetTenants verifies that getTenants encodes the 16-byte time range correctly.
func TestHTTPSelectNodeGetTenants(t *testing.T) {
	wantTenants := []string{"1:0", "2:0"}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"tenants" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 16 {
			t.Errorf("expected 16-byte body, got %d bytes", len(body))
			http.Error(w, "bad size", http.StatusBadRequest)
			return
		}
		minTs := encoding.UnmarshalInt64(body[:8])
		maxTs := encoding.UnmarshalInt64(body[8:16])
		if minTs != 1000 || maxTs != 2000 {
			t.Errorf("expected tr=(1000,2000), got (%d,%d)", minTs, maxTs)
		}
		writeStringsResponse(w, wantTenants, false)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	tr := storage.TimeRange{MinTimestamp: 1000, MaxTimestamp: 2000}
	tenants, err := sn.getTenants(testQT(), tr, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !reflect.DeepEqual(tenants, wantTenants) {
		t.Errorf("tenants: got %v, want %v", tenants, wantTenants)
	}
}

// TestHTTPSelectNodeGetSeriesCount verifies the 8-byte accountID+projectID encoding.
func TestHTTPSelectNodeGetSeriesCount(t *testing.T) {
	wantCount := uint64(12345)
	wantPartial := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"seriesCount" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) != 8 {
			t.Errorf("expected 8-byte body, got %d bytes", len(body))
			http.Error(w, "bad size", http.StatusBadRequest)
			return
		}
		accountID := encoding.UnmarshalUint32(body[:4])
		projectID := encoding.UnmarshalUint32(body[4:8])
		if accountID != 7 || projectID != 9 {
			t.Errorf("expected accountID=7, projectID=9; got %d, %d", accountID, projectID)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct { //nolint:errcheck
			SeriesCount uint64 `json:"seriesCount"`
			IsPartial   bool   `json:"isPartial"`
		}{SeriesCount: wantCount, IsPartial: wantPartial})
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	count, isPartial, err := sn.getSeriesCount(testQT(), 7, 9, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if count != wantCount {
		t.Errorf("count: got %d, want %d", count, wantCount)
	}
	if isPartial != wantPartial {
		t.Errorf("isPartial: got %v, want %v", isPartial, wantPartial)
	}
}

// TestHTTPSelectNodeGetSearchMetricNames verifies that getSearchMetricNames sends requestData directly.
func TestHTTPSelectNodeGetSearchMetricNames(t *testing.T) {
	wantMetrics := []string{`{__name__="up",job="node"}`, `{__name__="go_goroutines"}`}
	requestData := makeSearchQueryData(1, 0)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"searchMetricNames" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !reflect.DeepEqual(body, requestData) {
			t.Errorf("unexpected request body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct { //nolint:errcheck
			MetricNames []string `json:"metricNames"`
			IsPartial   bool     `json:"isPartial"`
		}{MetricNames: wantMetrics, IsPartial: false})
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	names, isPartial, err := sn.getSearchMetricNames(testQT(), requestData, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false")
	}
	if !reflect.DeepEqual(names, wantMetrics) {
		t.Errorf("metric names: got %v, want %v", names, wantMetrics)
	}
}

// TestHTTPSelectNodeDeleteSeries verifies deleteSeries returns the deleted count.
func TestHTTPSelectNodeDeleteSeries(t *testing.T) {
	wantDeleted := 42

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"deleteSeries" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct { //nolint:errcheck
			DeletedCount int `json:"deletedCount"`
		}{DeletedCount: wantDeleted})
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	n, err := sn.deleteSeries(testQT(), makeSearchQueryData(1, 0), testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if n != wantDeleted {
		t.Errorf("deletedCount: got %d, want %d", n, wantDeleted)
	}
}

// TestHTTPSelectNodeRegisterMetricNames verifies that registerMetricNames encodes MetricRows correctly.
func TestHTTPSelectNodeRegisterMetricNames(t *testing.T) {
	wantRows := []storage.MetricRow{
		{MetricNameRaw: []byte("metric1"), Timestamp: 1000},
		{MetricNameRaw: []byte("metric2"), Timestamp: 2000},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"registerMetricNames" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// First 8 bytes = uint64(count)
		count := int(encoding.UnmarshalUint64(body[:8]))
		if count != len(wantRows) {
			t.Errorf("expected count=%d, got %d", len(wantRows), count)
		}
		body = body[8:]
		for i, wantRow := range wantRows {
			nameLen := int(encoding.UnmarshalUint64(body[:8]))
			body = body[8:]
			name := body[:nameLen]
			body = body[nameLen:]
			ts := int64(encoding.UnmarshalUint64(body[:8]))
			body = body[8:]
			if !reflect.DeepEqual(name, wantRow.MetricNameRaw) {
				t.Errorf("row[%d] name: got %q, want %q", i, name, wantRow.MetricNameRaw)
			}
			if ts != wantRow.Timestamp {
				t.Errorf("row[%d] timestamp: got %d, want %d", i, ts, wantRow.Timestamp)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	if err := sn.registerMetricNames(testQT(), wantRows, testHTTPDeadline()); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

// TestHTTPSelectNodeGetTagValueSuffixes verifies body encoding for getTagValueSuffixes.
func TestHTTPSelectNodeGetTagValueSuffixes(t *testing.T) {
	wantSuffixes := []string{"host1", "host2"}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"tagValueSuffixes" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// uint32(accountID) + uint32(projectID) + int64(min) + int64(max) = 24 bytes
		if len(body) < 24 {
			t.Errorf("body too short: %d bytes", len(body))
			http.Error(w, "too short", http.StatusBadRequest)
			return
		}
		accountID := encoding.UnmarshalUint32(body[:4])
		projectID := encoding.UnmarshalUint32(body[4:8])
		minTs := encoding.UnmarshalInt64(body[8:16])
		maxTs := encoding.UnmarshalInt64(body[16:24])
		body = body[24:]
		if accountID != 3 || projectID != 5 {
			t.Errorf("expected accountID=3, projectID=5; got %d, %d", accountID, projectID)
		}
		if minTs != 100 || maxTs != 200 {
			t.Errorf("expected tr=(100,200), got (%d,%d)", minTs, maxTs)
		}
		// uint64(len(tagKey)) + tagKey
		tagKeyLen := int(encoding.UnmarshalUint64(body[:8]))
		body = body[8:]
		tagKey := string(body[:tagKeyLen])
		body = body[tagKeyLen:]
		if tagKey != "env" {
			t.Errorf("expected tagKey=%q, got %q", "env", tagKey)
		}
		// uint64(len(tagValuePrefix)) + tagValuePrefix
		prefixLen := int(encoding.UnmarshalUint64(body[:8]))
		body = body[8:]
		prefix := string(body[:prefixLen])
		body = body[prefixLen:]
		if prefix != "prod-" {
			t.Errorf("expected prefix=%q, got %q", "prod-", prefix)
		}
		// delimiter byte + uint32(maxSuffixes)
		delimiter := body[0]
		maxSuffixes := int(encoding.UnmarshalUint32(body[1:5]))
		if delimiter != '.' {
			t.Errorf("expected delimiter='.', got %q", delimiter)
		}
		if maxSuffixes != 20 {
			t.Errorf("expected maxSuffixes=20, got %d", maxSuffixes)
		}
		writeStringsResponse(w, wantSuffixes, false)
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	tr := storage.TimeRange{MinTimestamp: 100, MaxTimestamp: 200}
	suffixes, isPartial, err := sn.getTagValueSuffixes(testQT(), 3, 5, tr, "env", "prod-", '.', 20, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false")
	}
	if !reflect.DeepEqual(suffixes, wantSuffixes) {
		t.Errorf("suffixes: got %v, want %v", suffixes, wantSuffixes)
	}
}

// TestHTTPSelectNodeProcessSearchQuery verifies that processSearchQuery reads the binary stream.
func TestHTTPSelectNodeProcessSearchQuery(t *testing.T) {
	block1 := []byte("block-data-1")
	block2 := []byte("block-data-2")
	wantPartial := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != httpInternalPath+"search" {
			t.Errorf("unexpected path %q", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set(httpHeaderPartialResponse, "true")
		// Write binary stream: [uint64(size) + data]... + uint64(0)
		var buf []byte
		buf = encoding.MarshalUint64(buf, uint64(len(block1)))
		buf = append(buf, block1...)
		buf = encoding.MarshalUint64(buf, uint64(len(block2)))
		buf = append(buf, block2...)
		buf = encoding.MarshalUint64(buf, 0) // end-of-stream
		w.Write(buf)                         //nolint:errcheck
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	var gotBlocks [][]byte
	isPartial, err := sn.processSearchQuery(testQT(), makeSearchQueryData(1, 0), func(rawBlock []byte, _ uint) error {
		gotBlocks = append(gotBlocks, append([]byte{}, rawBlock...))
		return nil
	}, 0, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial != wantPartial {
		t.Errorf("isPartial: got %v, want %v", isPartial, wantPartial)
	}
	if len(gotBlocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(gotBlocks))
	}
	if !reflect.DeepEqual(gotBlocks[0], block1) {
		t.Errorf("block[0]: got %q, want %q", gotBlocks[0], block1)
	}
	if !reflect.DeepEqual(gotBlocks[1], block2) {
		t.Errorf("block[1]: got %q, want %q", gotBlocks[1], block2)
	}
}

// TestHTTPSelectNodeProcessSearchQueryEmpty verifies that an empty stream (only end-of-stream marker)
// returns no blocks.
func TestHTTPSelectNodeProcessSearchQueryEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set(httpHeaderPartialResponse, "false")
		buf := encoding.MarshalUint64(nil, 0) // only end-of-stream
		w.Write(buf)                          //nolint:errcheck
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	var gotBlocks [][]byte
	isPartial, err := sn.processSearchQuery(testQT(), makeSearchQueryData(1, 0), func(rawBlock []byte, _ uint) error {
		gotBlocks = append(gotBlocks, rawBlock)
		return nil
	}, 0, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false")
	}
	if len(gotBlocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(gotBlocks))
	}
}

// TestHTTPSelectNodeDeadlineExceeded verifies that a zero deadline returns an error immediately.
func TestHTTPSelectNodeDeadlineExceeded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when deadline is already exceeded")
	}))
	defer ts.Close()

	sn := testHTTPNode(ts)
	// Deadline in the past.
	pastDeadline := searchutil.DeadlineFromTimestamp(0)
	_, _, err := sn.getLabelNames(testQT(), makeSearchQueryData(1, 0), 10, pastDeadline)
	if err == nil {
		t.Fatal("expected error for exceeded deadline, got nil")
	}
}
