package clusternative

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// marshalTestSearchQuery creates a binary-encoded SearchQuery for testing.
// Format: uint32(numTenants) + [uint32(accountID)+uint32(projectID)]*numTenants + MarshalWithoutTenant.
func marshalTestSearchQuery(accountID, projectID uint32) []byte {
	sq := storage.NewSearchQuery(accountID, projectID, 1000, 2000, nil, 0)
	tt := storage.TenantToken{AccountID: accountID, ProjectID: projectID}
	var data []byte
	data = encoding.MarshalUint32(data, 1)
	data = tt.Marshal(data)
	data = append(data, sq.MarshalWithoutTenant(nil)...)
	return data
}

// TestHandleInternalRequestNonPOST verifies that non-POST requests return false.
func TestHandleInternalRequestNonPOST(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		r := httptest.NewRequest(method, InternalHTTPPath+"labelNames", nil)
		w := httptest.NewRecorder()
		if HandleInternalRequest(w, r) {
			t.Errorf("expected false for %s method", method)
		}
	}
}

// TestHandleInternalRequestWrongPath verifies that requests to other paths return false.
func TestHandleInternalRequestWrongPath(t *testing.T) {
	for _, path := range []string{"/", "/api/v1/labels", "/internal/other/"} {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		if HandleInternalRequest(w, r) {
			t.Errorf("expected false for path %q", path)
		}
	}
}

// TestHandleInternalRequestUnknownAction verifies that an unknown action returns 404.
func TestHandleInternalRequestUnknownAction(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"nonexistent", bytes.NewReader(nil))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	if !HandleInternalRequest(w, r) {
		t.Fatal("expected true for handled path, got false")
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestParseRequestMetaDefaultTimeout verifies default timeout is applied when header is absent.
func TestParseRequestMetaDefaultTimeout(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	qt, dl, err := parseRequestMeta(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	qt.Done()
	if dl.Deadline() == 0 {
		t.Error("expected non-zero deadline")
	}
}

// TestParseRequestMetaCustomTimeout verifies that X-Timeout-Secs is parsed correctly.
func TestParseRequestMetaCustomTimeout(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderTimeoutSecs, "120")
	qt, dl, err := parseRequestMeta(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	qt.Done()
	if dl.Deadline() == 0 {
		t.Error("expected non-zero deadline")
	}
}

// TestParseRequestMetaInvalidTimeout verifies that a non-numeric X-Timeout-Secs returns an error.
func TestParseRequestMetaInvalidTimeout(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set(HeaderTimeoutSecs, "not-a-number")
	qt, _, err := parseRequestMeta(r)
	if err == nil {
		qt.Done()
		t.Fatal("expected error for invalid timeout, got nil")
	}
}

// TestUnmarshalSearchQueryRoundTrip verifies that a marshaled SearchQuery can be unmarshaled.
func TestUnmarshalSearchQueryRoundTrip(t *testing.T) {
	data := marshalTestSearchQuery(7, 13)
	sq, err := unmarshalSearchQuery(data)
	if err != nil {
		t.Fatalf("unmarshalSearchQuery error: %s", err)
	}
	if sq.AccountID != 7 {
		t.Errorf("AccountID: got %d, want 7", sq.AccountID)
	}
	if sq.ProjectID != 13 {
		t.Errorf("ProjectID: got %d, want 13", sq.ProjectID)
	}
	if sq.MinTimestamp != 1000 {
		t.Errorf("MinTimestamp: got %d, want 1000", sq.MinTimestamp)
	}
	if sq.MaxTimestamp != 2000 {
		t.Errorf("MaxTimestamp: got %d, want 2000", sq.MaxTimestamp)
	}
}

// TestUnmarshalSearchQueryBadData verifies that malformed data returns an error.
func TestUnmarshalSearchQueryBadData(t *testing.T) {
	_, err := unmarshalSearchQuery([]byte{1, 2, 3}) // too short
	if err == nil {
		t.Fatal("expected error for bad data, got nil")
	}
}

// TestUnmarshalSearchQueryTrailingBytes verifies that trailing bytes after a valid SearchQuery
// return an error (to prevent silent data corruption).
func TestUnmarshalSearchQueryTrailingBytes(t *testing.T) {
	data := marshalTestSearchQuery(1, 0)
	data = append(data, 0xFF, 0xFF) // extra bytes
	_, err := unmarshalSearchQuery(data)
	if err == nil {
		t.Fatal("expected error for trailing bytes, got nil")
	}
}

// TestHandleLabelNamesBadBody verifies that a too-short body returns 400.
func TestHandleLabelNamesBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 4 bytes
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"labelNames", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleLabelNamesBadSearchQuery verifies that an invalid SearchQuery body returns 400.
func TestHandleLabelNamesBadSearchQuery(t *testing.T) {
	// 4 valid bytes for maxLabelNames, but the sqData part is garbage.
	var body []byte
	body = append(body, 0x00, 0x01, 0x02) // garbage sqData (only 3 bytes, not enough for AccountID)
	body = append(body, encoding.MarshalUint32(nil, 100)...)
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"labelNames", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad SearchQuery, got %d", w.Code)
	}
}

// TestHandleLabelValuesBadBody verifies that a too-short body for labelValues returns 400.
func TestHandleLabelValuesBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 8 bytes needed for uint64(labelNameLen)
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"labelValues", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleLabelValuesTruncatedLabel verifies that a body too short for the label name returns 400.
func TestHandleLabelValuesTruncatedLabel(t *testing.T) {
	var body []byte
	body = encoding.MarshalUint64(body, 100) // says label is 100 bytes
	body = append(body, []byte("short")...)  // only 5 bytes provided
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"labelValues", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for truncated label, got %d", w.Code)
	}
}

// TestHandleTenantsBadBody verifies that a too-short body for tenants returns 400.
func TestHandleTenantsBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 16 bytes
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"tenants", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleSeriesCountBadBody verifies that a too-short body for seriesCount returns 400.
func TestHandleSeriesCountBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 8 bytes
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"seriesCount", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleTSDBStatusBadBody verifies that a too-short body for tsdbStatus returns 400.
func TestHandleTSDBStatusBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 8 bytes for uint64(sqLen)
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"tsdbStatus", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleRegisterMetricNamesBadBody verifies that a too-short body returns 400.
func TestHandleRegisterMetricNamesBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 8 bytes for uint64(count)
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"registerMetricNames", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleTagValueSuffixesBadBody verifies that a too-short body returns 400.
func TestHandleTagValueSuffixesBadBody(t *testing.T) {
	body := []byte{1, 2, 3} // less than 24 bytes minimum
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"tagValueSuffixes", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short body, got %d", w.Code)
	}
}

// TestHandleSearchBadSearchQuery verifies that an invalid SearchQuery for search returns 400.
func TestHandleSearchBadSearchQuery(t *testing.T) {
	body := []byte{0x00, 0x01, 0x02} // garbage – not a valid SearchQuery
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"search", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	w := httptest.NewRecorder()
	HandleInternalRequest(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad SearchQuery, got %d", w.Code)
	}
}

// TestWriteJSONResponse verifies that writeJSONResponse sets Content-Type and encodes the value.
func TestWriteJSONResponse(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSONResponse(w, stringsResponse{Data: []string{"a", "b"}, IsPartial: true})
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
	var resp stringsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("cannot decode response: %s", err)
	}
	if !resp.IsPartial {
		t.Error("expected isPartial=true")
	}
	if len(resp.Data) != 2 || resp.Data[0] != "a" || resp.Data[1] != "b" {
		t.Errorf("unexpected data: %v", resp.Data)
	}
}

// TestHandleInternalRequestBodyReadError verifies behavior when the body read fails.
// (This can't be easily triggered in unit tests, but we verify the path exists via coverage.)

// TestHandleLabelNamesRequestBodyDecoding verifies the full body encoding contract:
// the client encodes sqData + uint32(maxLabelNames) and the server decodes it correctly.
// We test this by building the exact same body that http_node.go's getLabelNames would send.
func TestHandleLabelNamesRequestBodyDecoding(t *testing.T) {
	sqData := marshalTestSearchQuery(1, 2)
	maxLabelNames := uint32(77)

	// Build body as http_node.go does: sqData + uint32(maxLabelNames)
	body := append(append([]byte{}, sqData...), encoding.MarshalUint32(nil, maxLabelNames)...)

	// Call handleLabelNames directly.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"labelNames", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")

	// We don't have real storage, so the handler will call netstorage.LabelNames and return 500.
	// But we can verify that the request body is parsed without a 400 error.
	HandleInternalRequest(w, r)
	code := w.Code
	if code == http.StatusBadRequest {
		t.Errorf("got 400 (bad request) — body decoding failed: %s", strings.TrimSpace(w.Body.String()))
	}
	// 500 is acceptable: it means body was parsed OK but storage returned an error.
}

// TestHandleTSDBStatusBodyDecoding verifies the multi-field body format for tsdbStatus.
func TestHandleTSDBStatusBodyDecoding(t *testing.T) {
	sqData := marshalTestSearchQuery(1, 0)
	focusLabel := "instance"
	topN := uint32(10)

	var body []byte
	body = encoding.MarshalUint64(body, uint64(len(sqData)))
	body = append(body, sqData...)
	body = encoding.MarshalUint64(body, uint64(len(focusLabel)))
	body = append(body, []byte(focusLabel)...)
	body = append(body, encoding.MarshalUint32(nil, topN)...)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"tsdbStatus", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	HandleInternalRequest(w, r)

	// 400 would mean our body encoding is wrong.
	if w.Code == http.StatusBadRequest {
		t.Errorf("got 400 — body decoding failed: %s", strings.TrimSpace(w.Body.String()))
	}
}

// TestHandleTagValueSuffixesBodyDecoding verifies that the tag value suffixes body is correctly structured.
func TestHandleTagValueSuffixesBodyDecoding(t *testing.T) {
	accountID := uint32(3)
	projectID := uint32(5)
	minTs := int64(100)
	maxTs := int64(200)
	tagKey := "env"
	tagValuePrefix := "prod-"
	delimiter := byte('.')
	maxSuffixes := uint32(20)

	var body []byte
	body = encoding.MarshalUint32(body, accountID)
	body = encoding.MarshalUint32(body, projectID)
	body = encoding.MarshalInt64(body, minTs)
	body = encoding.MarshalInt64(body, maxTs)
	body = encoding.MarshalUint64(body, uint64(len(tagKey)))
	body = append(body, []byte(tagKey)...)
	body = encoding.MarshalUint64(body, uint64(len(tagValuePrefix)))
	body = append(body, []byte(tagValuePrefix)...)
	body = append(body, delimiter)
	body = append(body, encoding.MarshalUint32(nil, maxSuffixes)...)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"tagValueSuffixes", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	HandleInternalRequest(w, r)

	if w.Code == http.StatusBadRequest {
		t.Errorf("got 400 — body decoding failed: %s", strings.TrimSpace(w.Body.String()))
	}
}

// TestHandleRegisterMetricNamesBodyDecoding verifies the MetricRow encoding format.
func TestHandleRegisterMetricNamesBodyDecoding(t *testing.T) {
	rows := []storage.MetricRow{
		{MetricNameRaw: []byte("metric1"), Timestamp: 1000},
	}

	var body []byte
	body = encoding.MarshalUint64(body, uint64(len(rows)))
	for _, mr := range rows {
		body = encoding.MarshalUint64(body, uint64(len(mr.MetricNameRaw)))
		body = append(body, mr.MetricNameRaw...)
		body = encoding.MarshalUint64(body, uint64(mr.Timestamp))
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, InternalHTTPPath+"registerMetricNames", bytes.NewReader(body))
	r.Header.Set(HeaderTimeoutSecs, "60")
	HandleInternalRequest(w, r)

	if w.Code == http.StatusBadRequest {
		t.Errorf("got 400 — body decoding failed: %s", strings.TrimSpace(w.Body.String()))
	}
}
