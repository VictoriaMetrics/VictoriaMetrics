package clusternative

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// HTTP headers used in internal clusternative communication.
const (
	HeaderPartialResponse = "X-Partial-Response"
	HeaderTraceJSON       = "X-Trace-JSON"
	HeaderTimeoutSecs     = "X-Timeout-Secs"
	HeaderTraceEnabled    = "X-Trace-Enabled"
)

// InternalHTTPPath is the URL path prefix for internal clusternative HTTP endpoints.
const InternalHTTPPath = "/internal/clusternative/"

// HandleInternalRequest handles internal clusternative HTTP requests.
// It returns true if the request was handled.
func HandleInternalRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, InternalHTTPPath) {
		return false
	}
	action := path[len(InternalHTTPPath):]

	qt, deadline, err := parseRequestMeta(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot parse request metadata: %s", err), http.StatusBadRequest)
		return true
	}
	defer qt.Done()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot read request body: %s", err), http.StatusBadRequest)
		return true
	}

	switch action {
	case "labelNames":
		handleLabelNames(w, qt, body, deadline)
	case "labelValues":
		handleLabelValues(w, qt, body, deadline)
	case "tenants":
		handleTenants(w, qt, body, deadline)
	case "seriesCount":
		handleSeriesCount(w, qt, body, deadline)
	case "tsdbStatus":
		handleTSDBStatus(w, qt, body, deadline)
	case "searchMetricNames":
		handleSearchMetricNames(w, qt, body, deadline)
	case "deleteSeries":
		handleDeleteSeries(w, qt, body, deadline)
	case "registerMetricNames":
		handleRegisterMetricNames(w, qt, body, deadline)
	case "tagValueSuffixes":
		handleTagValueSuffixes(w, qt, body, deadline)
	case "search":
		handleSearch(w, qt, body, deadline)
	default:
		http.Error(w, fmt.Sprintf("unsupported action: %q", action), http.StatusNotFound)
	}
	return true
}

func parseRequestMeta(r *http.Request) (*querytracer.Tracer, searchutil.Deadline, error) {
	traceEnabled := r.Header.Get(HeaderTraceEnabled) == "true"
	qt := querytracer.New(traceEnabled, "http clusternative request %s", r.URL.Path)

	timeoutStr := r.Header.Get(HeaderTimeoutSecs)
	timeoutSecs := uint64(60)
	if timeoutStr != "" {
		n, err := strconv.ParseUint(timeoutStr, 10, 64)
		if err != nil {
			qt.Done()
			return nil, searchutil.Deadline{}, fmt.Errorf("cannot parse %s header %q: %w", HeaderTimeoutSecs, timeoutStr, err)
		}
		timeoutSecs = n
	}
	deadline := searchutil.DeadlineFromTimestamp(fasttime.UnixTimestamp() + timeoutSecs)
	return qt, deadline, nil
}

func writeTraceJSON(w http.ResponseWriter, qt *querytracer.Tracer) {
	traceJSON := qt.ToJSON()
	if traceJSON != "" {
		w.Header().Set(HeaderTraceJSON, traceJSON)
	}
}

// unmarshalSearchQuery parses a SearchQuery from binary requestData.
//
// Wire format: uint32(numTenants) + [uint32(accountID)+uint32(projectID)] * numTenants + MarshalWithoutTenant
func unmarshalSearchQuery(data []byte) (*storage.SearchQuery, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("cannot unmarshal numTenants: data too short (%d bytes)", len(data))
	}
	numTenants := int(encoding.UnmarshalUint32(data[:4]))
	data = data[4:]

	if numTenants <= 0 {
		return nil, fmt.Errorf("unexpected numTenants=%d; must be > 0", numTenants)
	}
	if len(data) < numTenants*8 {
		return nil, fmt.Errorf("cannot unmarshal %d tenant tokens: data too short (%d bytes)", numTenants, len(data))
	}

	tokens := make([]storage.TenantToken, numTenants)
	for i := range numTenants {
		tokens[i] = storage.TenantToken{
			AccountID: encoding.UnmarshalUint32(data[:4]),
			ProjectID: encoding.UnmarshalUint32(data[4:8]),
		}
		data = data[8:]
	}

	// sq.Unmarshal expects [accountID(4B) + projectID(4B) + MarshalWithoutTenant].
	// Prepend the first token so sq.Unmarshal can parse the query body.
	firstTokenData := tokens[0].Marshal(nil)
	fullData := append(firstTokenData, data...)

	var sq storage.SearchQuery
	tail, err := sq.Unmarshal(fullData)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal SearchQuery: %w", err)
	}
	if len(tail) > 0 {
		return nil, fmt.Errorf("unexpected trailing bytes after SearchQuery: %d bytes", len(tail))
	}

	sq.TenantTokens = tokens
	if numTenants > 1 {
		sq.IsMultiTenant = true
	}
	return &sq, nil
}

type stringsResponse struct {
	Data      []string `json:"data"`
	IsPartial bool     `json:"isPartial"`
}

func writeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Errorf("cannot write JSON response: %s", err)
	}
}

func handleLabelNames(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: sqData (variable) + uint32(maxLabelNames) [4 bytes at end]
	if len(body) < 4 {
		http.Error(w, "request body too short for labelNames", http.StatusBadRequest)
		return
	}
	sqData := body[:len(body)-4]
	maxLabelNames := int(encoding.UnmarshalUint32(body[len(body)-4:]))

	sq, err := unmarshalSearchQuery(sqData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	labelNames, isPartial, err := netstorage.LabelNames(qt, false, sq, maxLabelNames, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, stringsResponse{Data: labelNames, IsPartial: isPartial})
}

func handleLabelValues(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: uint64(len(labelName)) + labelName + sqData + uint32(maxLabelValues) [4 bytes at end]
	if len(body) < 8 {
		http.Error(w, "request body too short for labelValues", http.StatusBadRequest)
		return
	}
	labelNameLen := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]
	if len(body) < labelNameLen+4 {
		http.Error(w, "request body too short for labelValues (label name + max)", http.StatusBadRequest)
		return
	}
	labelName := string(body[:labelNameLen])
	body = body[labelNameLen:]
	sqData := body[:len(body)-4]
	maxLabelValues := int(encoding.UnmarshalUint32(body[len(body)-4:]))

	sq, err := unmarshalSearchQuery(sqData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	labelValues, isPartial, err := netstorage.LabelValues(qt, false, labelName, sq, maxLabelValues, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, stringsResponse{Data: labelValues, IsPartial: isPartial})
}

func handleTenants(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: int64(minTimestamp) + int64(maxTimestamp) = 16 bytes
	if len(body) < 16 {
		http.Error(w, "request body too short for tenants", http.StatusBadRequest)
		return
	}
	minTs := encoding.UnmarshalInt64(body[:8])
	maxTs := encoding.UnmarshalInt64(body[8:16])
	tr := storage.TimeRange{MinTimestamp: minTs, MaxTimestamp: maxTs}

	tenants, err := netstorage.Tenants(qt, tr, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Tenants always denies partial responses internally.
	writeJSONResponse(w, stringsResponse{Data: tenants, IsPartial: false})
}

type seriesCountResponse struct {
	SeriesCount uint64 `json:"seriesCount"`
	IsPartial   bool   `json:"isPartial"`
}

func handleSeriesCount(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: uint32(accountID) + uint32(projectID) = 8 bytes
	if len(body) < 8 {
		http.Error(w, "request body too short for seriesCount", http.StatusBadRequest)
		return
	}
	accountID := encoding.UnmarshalUint32(body[:4])
	projectID := encoding.UnmarshalUint32(body[4:8])

	n, isPartial, err := netstorage.SeriesCount(qt, accountID, projectID, false, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, seriesCountResponse{SeriesCount: n, IsPartial: isPartial})
}

type tsdbStatusResponse struct {
	Status    *storage.TSDBStatus `json:"status"`
	IsPartial bool                `json:"isPartial"`
}

func handleTSDBStatus(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: sqData + uint64(len(focusLabel)) + focusLabel + uint32(topN)
	// Parse from start: sqData size is unknown, so we use a length-prefix approach.
	// Format: uint64(sqLen) + sqData + uint64(len(focusLabel)) + focusLabel + uint32(topN)
	if len(body) < 8 {
		http.Error(w, "request body too short for tsdbStatus", http.StatusBadRequest)
		return
	}
	sqLen := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]
	if len(body) < sqLen+8 {
		http.Error(w, "request body too short for tsdbStatus (sqData)", http.StatusBadRequest)
		return
	}
	sqData := body[:sqLen]
	body = body[sqLen:]

	focusLabelLen := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]
	if len(body) < focusLabelLen+4 {
		http.Error(w, "request body too short for tsdbStatus (focusLabel)", http.StatusBadRequest)
		return
	}
	focusLabel := string(body[:focusLabelLen])
	topN := int(encoding.UnmarshalUint32(body[focusLabelLen : focusLabelLen+4]))

	sq, err := unmarshalSearchQuery(sqData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status, isPartial, err := netstorage.TSDBStatus(qt, false, sq, focusLabel, topN, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, tsdbStatusResponse{Status: status, IsPartial: isPartial})
}

type metricNamesResponse struct {
	MetricNames []string `json:"metricNames"`
	IsPartial   bool     `json:"isPartial"`
}

func handleSearchMetricNames(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	sq, err := unmarshalSearchQuery(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metricNames, isPartial, err := netstorage.SearchMetricNames(qt, false, sq, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, metricNamesResponse{MetricNames: metricNames, IsPartial: isPartial})
}

type deleteSeriesResponse struct {
	DeletedCount int `json:"deletedCount"`
}

func handleDeleteSeries(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	sq, err := unmarshalSearchQuery(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	deletedCount, err := netstorage.DeleteSeries(qt, sq, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, deleteSeriesResponse{DeletedCount: deletedCount})
}

func handleRegisterMetricNames(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: uint64(count) + for each: uint64(len(metricNameRaw)) + metricNameRaw + int64(timestamp)
	if len(body) < 8 {
		http.Error(w, "request body too short for registerMetricNames", http.StatusBadRequest)
		return
	}
	count := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]

	mrs := make([]storage.MetricRow, 0, count)
	for i := range count {
		if len(body) < 8 {
			http.Error(w, fmt.Sprintf("cannot read metricNameRaw length for entry %d", i), http.StatusBadRequest)
			return
		}
		nameLen := int(encoding.UnmarshalUint64(body[:8]))
		body = body[8:]
		if len(body) < nameLen+8 {
			http.Error(w, fmt.Sprintf("cannot read metricNameRaw data for entry %d", i), http.StatusBadRequest)
			return
		}
		mr := storage.MetricRow{
			MetricNameRaw: append([]byte{}, body[:nameLen]...),
			Timestamp:     int64(encoding.UnmarshalUint64(body[nameLen : nameLen+8])),
		}
		body = body[nameLen+8:]
		mrs = append(mrs, mr)
	}

	if err := netstorage.RegisterMetricNames(qt, mrs, deadline); err != nil {
		writeTraceJSON(w, qt)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeTraceJSON(w, qt)
	w.WriteHeader(http.StatusNoContent)
}

func handleTagValueSuffixes(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	// Body: uint32(accountID) + uint32(projectID) + int64(min) + int64(max) +
	//       uint64(len(tagKey)) + tagKey + uint64(len(tagValuePrefix)) + tagValuePrefix +
	//       byte(delimiter) + uint32(maxSuffixes)
	if len(body) < 4+4+8+8 {
		http.Error(w, "request body too short for tagValueSuffixes", http.StatusBadRequest)
		return
	}
	accountID := encoding.UnmarshalUint32(body[:4])
	projectID := encoding.UnmarshalUint32(body[4:8])
	minTs := encoding.UnmarshalInt64(body[8:16])
	maxTs := encoding.UnmarshalInt64(body[16:24])
	tr := storage.TimeRange{MinTimestamp: minTs, MaxTimestamp: maxTs}
	body = body[24:]

	if len(body) < 8 {
		http.Error(w, "request body too short for tagValueSuffixes (tagKey len)", http.StatusBadRequest)
		return
	}
	tagKeyLen := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]
	if len(body) < tagKeyLen+8 {
		http.Error(w, "request body too short for tagValueSuffixes (tagKey + prefix len)", http.StatusBadRequest)
		return
	}
	tagKey := string(body[:tagKeyLen])
	body = body[tagKeyLen:]

	tagValuePrefixLen := int(encoding.UnmarshalUint64(body[:8]))
	body = body[8:]
	if len(body) < tagValuePrefixLen+1+4 {
		http.Error(w, "request body too short for tagValueSuffixes (tagValuePrefix + delimiter + maxSuffixes)", http.StatusBadRequest)
		return
	}
	tagValuePrefix := string(body[:tagValuePrefixLen])
	body = body[tagValuePrefixLen:]
	delimiter := body[0]
	maxSuffixes := int(encoding.UnmarshalUint32(body[1:5]))

	suffixes, isPartial, err := netstorage.TagValueSuffixes(qt, accountID, projectID, false, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
	writeTraceJSON(w, qt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSONResponse(w, stringsResponse{Data: suffixes, IsPartial: isPartial})
}

// handleSearch handles the streaming search request.
// Response format: sequence of [uint64(blockSize) + blockData] pairs, terminated by uint64(0).
// The X-Partial-Response header indicates whether the response is partial.
func handleSearch(w http.ResponseWriter, qt *querytracer.Tracer, body []byte, deadline searchutil.Deadline) {
	sq, err := unmarshalSearchQuery(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "server does not support streaming", http.StatusInternalServerError)
		return
	}

	// Collect all blocks first so we can set headers before streaming.
	var blocks [][]byte
	_, processBlocksFn := netstorage.PrepareProcessRawBlocks(qt, false, sq, deadline)
	isPartial, searchErr := processBlocksFn(func(rawBlock []byte, _ uint) error {
		blocks = append(blocks, append([]byte{}, rawBlock...))
		return nil
	})

	writeTraceJSON(w, qt)
	if searchErr != nil {
		http.Error(w, searchErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	if isPartial {
		w.Header().Set(HeaderPartialResponse, "true")
	} else {
		w.Header().Set(HeaderPartialResponse, "false")
	}

	sizeBuf := make([]byte, 8)
	for _, block := range blocks {
		sizeBuf = encoding.MarshalUint64(sizeBuf[:0], uint64(len(block)))
		if _, err := w.Write(sizeBuf); err != nil {
			logger.Errorf("cannot write block size to response: %s", err)
			return
		}
		if _, err := w.Write(block); err != nil {
			logger.Errorf("cannot write block to response: %s", err)
			return
		}
	}
	// Write end-of-stream marker.
	sizeBuf = encoding.MarshalUint64(sizeBuf[:0], 0)
	if _, err := w.Write(sizeBuf); err != nil {
		logger.Errorf("cannot write end-of-stream marker: %s", err)
		return
	}
	flusher.Flush()
}
