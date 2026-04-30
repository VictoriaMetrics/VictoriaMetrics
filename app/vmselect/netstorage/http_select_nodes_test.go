package netstorage

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// TestInitStopHTTPSelectNodes verifies that InitHTTPSelectNodes and
// MustStopHTTPSelectNodes can be called multiple times without panicking.
func TestInitStopHTTPSelectNodes(t *testing.T) {
	for range 3 {
		InitHTTPSelectNodes([]string{"localhost:8481", "localhost:8482"})
		MustStopHTTPSelectNodes()
	}
}

// TestInitHTTPSelectNodesEmpty verifies that Init with an empty slice is a no-op.
func TestInitHTTPSelectNodesEmpty(t *testing.T) {
	InitHTTPSelectNodes(nil)
	if nodes := getHTTPSelectNodes(); len(nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(nodes))
	}
}

// TestGetHTTPSelectNodesNilBeforeInit verifies getHTTPSelectNodes returns nil when uninitialized.
func TestGetHTTPSelectNodesNilBeforeInit(t *testing.T) {
	// Ensure clean state.
	MustStopHTTPSelectNodes()
	if nodes := getHTTPSelectNodes(); nodes != nil {
		t.Errorf("expected nil, got %d nodes", len(nodes))
	}
}

// TestLabelNamesFromHTTPNodesEmpty verifies that LabelNamesFromHTTPNodes returns empty results
// when no HTTP nodes are configured.
func TestLabelNamesFromHTTPNodesEmpty(t *testing.T) {
	MustStopHTTPSelectNodes()
	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)
	labels, isPartial, err := LabelNamesFromHTTPNodes(qt, false, sq, 100, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false when no nodes")
	}
	if len(labels) != 0 {
		t.Errorf("expected 0 labels, got %d", len(labels))
	}
}

// TestLabelNamesFromHTTPNodesMultipleNodes verifies that labels from multiple nodes are merged
// and deduplicated.
func TestLabelNamesFromHTTPNodesMultipleNodes(t *testing.T) {
	node1Labels := []string{"__name__", "env", "job"}
	node2Labels := []string{"job", "instance", "__name__"}
	wantLabels := []string{"__name__", "env", "instance", "job"}

	makeHandler := func(labels []string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jsonStringsResponse{Data: labels, IsPartial: false}) //nolint:errcheck
		}
	}
	ts1 := httptest.NewServer(makeHandler(node1Labels))
	defer ts1.Close()
	ts2 := httptest.NewServer(makeHandler(node2Labels))
	defer ts2.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts1.URL, ts2.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)
	labels, isPartial, err := LabelNamesFromHTTPNodes(qt, false, sq, 100, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false")
	}
	sort.Strings(labels)
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("labels: got %v, want %v", labels, wantLabels)
	}
}

// TestLabelNamesFromHTTPNodesPartialResponseAllowed verifies that when one node fails and
// denyPartialResponse=false, the result is marked partial but no error is returned.
func TestLabelNamesFromHTTPNodesPartialResponseAllowed(t *testing.T) {
	goodLabels := []string{"__name__", "job"}

	goodHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonStringsResponse{Data: goodLabels, IsPartial: false}) //nolint:errcheck
	})
	badHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "node down", http.StatusInternalServerError)
	})

	ts1 := httptest.NewServer(goodHandler)
	defer ts1.Close()
	ts2 := httptest.NewServer(badHandler)
	defer ts2.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts1.URL, ts2.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)
	labels, isPartial, err := LabelNamesFromHTTPNodes(qt, false, sq, 100, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error with denyPartialResponse=false: %s", err)
	}
	if !isPartial {
		t.Error("expected isPartial=true when one node fails")
	}
	sort.Strings(labels)
	if !reflect.DeepEqual(labels, goodLabels) {
		t.Errorf("labels: got %v, want %v", labels, goodLabels)
	}
}

// TestLabelNamesFromHTTPNodesDenyPartial verifies that when one node fails and
// denyPartialResponse=true, an error is returned.
func TestLabelNamesFromHTTPNodesDenyPartial(t *testing.T) {
	badHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "node down", http.StatusInternalServerError)
	})

	ts := httptest.NewServer(badHandler)
	defer ts.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)
	_, _, err := LabelNamesFromHTTPNodes(qt, true, sq, 100, testHTTPDeadline())
	if err == nil {
		t.Fatal("expected error with denyPartialResponse=true, got nil")
	}
}

// TestLabelNamesFromHTTPNodesIsPartialPropagated verifies that isPartial from a node is propagated.
func TestLabelNamesFromHTTPNodesIsPartialPropagated(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jsonStringsResponse{Data: []string{"up"}, IsPartial: true}) //nolint:errcheck
	}))
	defer ts.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)
	_, isPartial, err := LabelNamesFromHTTPNodes(qt, false, sq, 100, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !isPartial {
		t.Error("expected isPartial=true when node reports partial")
	}
}

// TestTenantsFromHTTPNodesMultipleNodes verifies fan-out and deduplication for TenantsFromHTTPNodes.
func TestTenantsFromHTTPNodesMultipleNodes(t *testing.T) {
	wantTenants := []string{"1:0", "2:0", "3:0"}

	makeHandler := func(tenants []string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if len(body) != 16 {
				http.Error(w, "bad body", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jsonStringsResponse{Data: tenants}) //nolint:errcheck
		}
	}
	ts1 := httptest.NewServer(makeHandler([]string{"1:0", "2:0"}))
	defer ts1.Close()
	ts2 := httptest.NewServer(makeHandler([]string{"2:0", "3:0"}))
	defer ts2.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts1.URL, ts2.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	tr := storage.TimeRange{MinTimestamp: 0, MaxTimestamp: 1000}
	tenants, err := TenantsFromHTTPNodes(qt, tr, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	sort.Strings(tenants)
	if !reflect.DeepEqual(tenants, wantTenants) {
		t.Errorf("tenants: got %v, want %v", tenants, wantTenants)
	}
}

// TestSeriesCountFromHTTPNodesMultipleNodes verifies that series counts from multiple nodes are summed.
func TestSeriesCountFromHTTPNodesMultipleNodes(t *testing.T) {
	makeHandler := func(count uint64, isPartial bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(struct { //nolint:errcheck
				SeriesCount uint64 `json:"seriesCount"`
				IsPartial   bool   `json:"isPartial"`
			}{SeriesCount: count, IsPartial: isPartial})
		}
	}
	ts1 := httptest.NewServer(makeHandler(1000, false))
	defer ts1.Close()
	ts2 := httptest.NewServer(makeHandler(500, false))
	defer ts2.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts1.URL, ts2.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	total, isPartial, err := SeriesCountFromHTTPNodes(qt, 1, 0, false, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if isPartial {
		t.Error("expected isPartial=false")
	}
	if total != 1500 {
		t.Errorf("total: got %d, want 1500", total)
	}
}

// TestMarshalSearchQueryData verifies that marshalSearchQueryData produces the correct
// wire format: uint32(numTenants) + TenantTokens + MarshalWithoutTenant.
func TestMarshalSearchQueryData(t *testing.T) {
	sq := storage.NewSearchQuery(7, 13, 1000, 9000, nil, 500)
	data := marshalSearchQueryData(sq)
	if len(data) == 0 {
		t.Fatal("expected non-empty marshaled data")
	}

	if len(data) < 4 {
		t.Fatal("data too short for numTenants")
	}
	numTenants := encoding.UnmarshalUint32(data[:4])
	if numTenants != 1 {
		t.Errorf("numTenants: got %d, want 1", numTenants)
	}
	data = data[4:]

	if len(data) < 8 {
		t.Fatal("data too short for tenant token")
	}
	accountID := encoding.UnmarshalUint32(data[:4])
	projectID := encoding.UnmarshalUint32(data[4:8])
	if accountID != 7 {
		t.Errorf("AccountID: got %d, want 7", accountID)
	}
	if projectID != 13 {
		t.Errorf("ProjectID: got %d, want 13", projectID)
	}
	data = data[8:]

	// Reconstruct full data for sq.Unmarshal: [accountID + projectID + MarshalWithoutTenant]
	fullData := encoding.MarshalUint32(nil, accountID)
	fullData = encoding.MarshalUint32(fullData, projectID)
	fullData = append(fullData, data...)

	var sq2 storage.SearchQuery
	tail, err := sq2.Unmarshal(fullData)
	if err != nil {
		t.Fatalf("Unmarshal error: %s", err)
	}
	if len(tail) != 0 {
		t.Errorf("unexpected trailing bytes: %d", len(tail))
	}
	if sq2.MinTimestamp != sq.MinTimestamp {
		t.Errorf("MinTimestamp: got %d, want %d", sq2.MinTimestamp, sq.MinTimestamp)
	}
	if sq2.MaxTimestamp != sq.MaxTimestamp {
		t.Errorf("MaxTimestamp: got %d, want %d", sq2.MaxTimestamp, sq.MaxTimestamp)
	}
	if sq2.MaxMetrics != sq.MaxMetrics {
		t.Errorf("MaxMetrics: got %d, want %d", sq2.MaxMetrics, sq.MaxMetrics)
	}
}

// TestMarshalSearchQueryDataEmptyTenants verifies that marshalSearchQueryData handles
// empty TenantTokens by using a zero-value TenantToken.
func TestMarshalSearchQueryDataEmptyTenants(t *testing.T) {
	sq := &storage.SearchQuery{MinTimestamp: 100, MaxTimestamp: 200}
	data := marshalSearchQueryData(sq)
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}

	if len(data) < 4 {
		t.Fatal("data too short for numTenants")
	}
	numTenants := encoding.UnmarshalUint32(data[:4])
	if numTenants != 1 {
		t.Errorf("numTenants: got %d, want 1", numTenants)
	}
	data = data[4:]

	if len(data) < 8 {
		t.Fatal("data too short for tenant token")
	}
	accountID := encoding.UnmarshalUint32(data[:4])
	projectID := encoding.UnmarshalUint32(data[4:8])
	if accountID != 0 || projectID != 0 {
		t.Errorf("expected AccountID=0, ProjectID=0; got %d, %d", accountID, projectID)
	}
}

// TestProcessSearchQueryOnHTTPNodesMultipleNodes verifies fan-out and block streaming across nodes.
func TestProcessSearchQueryOnHTTPNodesMultipleNodes(t *testing.T) {
	block1 := []byte("node1-block")
	block2 := []byte("node2-block")

	makeSearchHandler := func(block []byte, isPartial bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			partialVal := "false"
			if isPartial {
				partialVal = "true"
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set(httpHeaderPartialResponse, partialVal)
			var buf []byte
			buf = encoding.MarshalUint64(buf, uint64(len(block)))
			buf = append(buf, block...)
			buf = encoding.MarshalUint64(buf, 0)
			w.Write(buf) //nolint:errcheck
		}
	}

	ts1 := httptest.NewServer(makeSearchHandler(block1, false))
	defer ts1.Close()
	ts2 := httptest.NewServer(makeSearchHandler(block2, true))
	defer ts2.Close()

	MustStopHTTPSelectNodes()
	InitHTTPSelectNodes([]string{ts1.URL, ts2.URL})
	defer MustStopHTTPSelectNodes()

	qt := querytracer.New(false, "test")
	sq := storage.NewSearchQuery(1, 0, 0, 1000, nil, 0)

	var gotBlocks [][]byte
	isPartial, err := ProcessSearchQueryOnHTTPNodes(qt, false, sq, func(rawBlock []byte, _ uint) error {
		gotBlocks = append(gotBlocks, append([]byte{}, rawBlock...))
		return nil
	}, testHTTPDeadline())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !isPartial {
		t.Error("expected isPartial=true (one node reported partial)")
	}
	if len(gotBlocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(gotBlocks))
	}
}
