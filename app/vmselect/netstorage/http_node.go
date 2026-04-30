package netstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

// HTTP headers for internal clusternative communication.
// These mirror the constants in clusternative/http_server.go but are
// defined here to avoid an import cycle (netstorage ← clusternative ← netstorage).
const (
	httpHeaderPartialResponse = "X-Partial-Response"
	httpHeaderTraceJSON       = "X-Trace-JSON"
	httpHeaderTimeoutSecs     = "X-Timeout-Secs"
	httpHeaderTraceEnabled    = "X-Trace-Enabled"
	httpInternalPath          = "/internal/clusternative/"
)

// httpSelectNode represents a vmselect node that is queried via HTTP
// instead of the legacy TCP-based RPC protocol.
// This is used for multi-level cluster setups where top-level vmselect
// communicates with lower-level vmselect nodes.
type httpSelectNode struct {
	// baseURL is the HTTP base URL of the lower-level vmselect node,
	// e.g. "vmselect-lower:8481".
	baseURL string

	// httpClient is used for sending HTTP requests.
	httpClient *http.Client

	// metrics
	concurrentQueries *metrics.Counter

	labelNamesRequests  *metrics.Counter
	labelNamesErrors    *metrics.Counter
	labelValuesRequests *metrics.Counter
	labelValuesErrors   *metrics.Counter
	searchRequests      *metrics.Counter
	searchErrors        *metrics.Counter
	tenantsRequests     *metrics.Counter
	tenantsErrors       *metrics.Counter
	seriesCountRequests *metrics.Counter
	seriesCountErrors   *metrics.Counter
	tsdbStatusRequests  *metrics.Counter
	tsdbStatusErrors    *metrics.Counter
	metricNamesRequests *metrics.Counter
	metricNamesErrors   *metrics.Counter
	deleteRequests      *metrics.Counter
	deleteErrors        *metrics.Counter
	registerRequests    *metrics.Counter
	registerErrors      *metrics.Counter
	tagSuffixRequests   *metrics.Counter
	tagSuffixErrors     *metrics.Counter
	metricBlocksRead    *metrics.Counter
}

// newHTTPSelectNode creates a new httpSelectNode for the given URL.
func newHTTPSelectNode(ms *metrics.Set, baseURL string) *httpSelectNode {
	baseURL = strings.TrimRight(baseURL, "/")
	return &httpSelectNode{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 0, // no global timeout; per-request deadline is used
		},

		concurrentQueries: ms.NewCounter(fmt.Sprintf(`vm_concurrent_queries{name="vmselect_http", addr=%q}`, baseURL)),

		labelNamesRequests:  ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		labelNamesErrors:    ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		labelValuesRequests: ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="labelValues", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		labelValuesErrors:   ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="labelValues", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		searchRequests:      ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="search", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		searchErrors:        ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="search", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tenantsRequests:     ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tenants", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tenantsErrors:       ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tenants", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		seriesCountRequests: ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="seriesCount", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		seriesCountErrors:   ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="seriesCount", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tsdbStatusRequests:  ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tsdbStatus", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tsdbStatusErrors:    ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tsdbStatus", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		metricNamesRequests: ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="searchMetricNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		metricNamesErrors:   ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="searchMetricNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		deleteRequests:      ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="deleteSeries", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		deleteErrors:        ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="deleteSeries", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		registerRequests:    ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="registerMetricNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		registerErrors:      ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="registerMetricNames", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tagSuffixRequests:   ms.NewCounter(fmt.Sprintf(`vm_requests_total{action="tagValueSuffixes", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		tagSuffixErrors:     ms.NewCounter(fmt.Sprintf(`vm_request_errors_total{action="tagValueSuffixes", type="httpClient", name="vmselect", addr=%q}`, baseURL)),
		metricBlocksRead:    ms.NewCounter(fmt.Sprintf(`vm_metric_blocks_read_total{name="vmselect_http", addr=%q}`, baseURL)),
	}
}

// addr returns the base URL of the node, used for logging and metrics.
func (sn *httpSelectNode) addr() string {
	return sn.baseURL
}

// doPost sends a POST request to the given endpoint with the given body.
// It sets required headers and returns the response.
func (sn *httpSelectNode) doPost(qt *querytracer.Tracer, action string, body []byte, deadline searchutil.Deadline) (*http.Response, error) {
	url := buildInternalURL(sn.baseURL, action)

	d := time.Unix(int64(deadline.Deadline()), 0)
	nowSecs := fasttime.UnixTimestamp()
	timeout := d.Sub(time.Unix(int64(nowSecs), 0))
	if timeout <= 0 {
		return nil, fmt.Errorf("request timeout reached for %s: %s", action, deadline.String())
	}

	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP request for %s: %w", action, err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(httpHeaderTimeoutSecs, strconv.FormatUint(uint64(timeout.Seconds()+1), 10))
	if qt.Enabled() {
		req.Header.Set(httpHeaderTraceEnabled, "true")
	}

	resp, err := sn.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot execute HTTP request for %s at %s: %w", action, url, err)
	}
	return resp, nil
}

func buildInternalURL(baseURL, action string) string {
	u := url.URL{
		Scheme: "http",
		Host:   baseURL,
		Path:   path.Join(httpInternalPath, action),
	}
	return u.String()
}

// readStringResponse reads a JSON stringsResponse from an HTTP response.
func readStringResponse(resp *http.Response, action string) ([]string, bool, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("unexpected HTTP status %d from %s: %s", resp.StatusCode, action, strings.TrimSpace(string(body)))
	}
	var result struct {
		Data      []string `json:"data"`
		IsPartial bool     `json:"isPartial"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("cannot decode JSON response from %s: %w", action, err)
	}
	return result.Data, result.IsPartial, nil
}

// getLabelNames returns label names matching the given requestData from the HTTP select node.
func (sn *httpSelectNode) getLabelNames(qt *querytracer.Tracer, requestData []byte, maxLabelNames int, deadline searchutil.Deadline) ([]string, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.labelNamesRequests.Inc()

	// Body: sqData + uint32(maxLabelNames)
	body := append(append([]byte{}, requestData...), encoding.MarshalUint32(nil, uint32(maxLabelNames))...)

	resp, err := sn.doPost(qt, "labelNames", body, deadline)
	if err != nil {
		sn.labelNamesErrors.Inc()
		return nil, false, err
	}
	readTraceFromResponse(qt, resp)
	labels, isPartial, err := readStringResponse(resp, "labelNames")
	if err != nil {
		sn.labelNamesErrors.Inc()
		return nil, false, fmt.Errorf("cannot get label names from vmselect %s: %w", sn.baseURL, err)
	}
	return labels, isPartial, nil
}

// getLabelValues returns label values for the given label name from the HTTP select node.
func (sn *httpSelectNode) getLabelValues(qt *querytracer.Tracer, labelName string, requestData []byte, maxLabelValues int, deadline searchutil.Deadline) ([]string, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.labelValuesRequests.Inc()

	// Body: uint64(len(labelName)) + labelName + sqData + uint32(maxLabelValues)
	labelNameBytes := []byte(labelName)
	var body []byte
	body = encoding.MarshalUint64(body, uint64(len(labelNameBytes)))
	body = append(body, labelNameBytes...)
	body = append(body, requestData...)
	body = append(body, encoding.MarshalUint32(nil, uint32(maxLabelValues))...)

	resp, err := sn.doPost(qt, "labelValues", body, deadline)
	if err != nil {
		sn.labelValuesErrors.Inc()
		return nil, false, err
	}
	readTraceFromResponse(qt, resp)
	values, isPartial, err := readStringResponse(resp, "labelValues")
	if err != nil {
		sn.labelValuesErrors.Inc()
		return nil, false, fmt.Errorf("cannot get label values from vmselect %s: %w", sn.baseURL, err)
	}
	return values, isPartial, nil
}

// getTenants returns tenants from the HTTP select node for the given time range.
func (sn *httpSelectNode) getTenants(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutil.Deadline) ([]string, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.tenantsRequests.Inc()

	// Body: int64(minTimestamp) + int64(maxTimestamp) = 16 bytes
	var body []byte
	body = encoding.MarshalInt64(body, tr.MinTimestamp)
	body = encoding.MarshalInt64(body, tr.MaxTimestamp)

	resp, err := sn.doPost(qt, "tenants", body, deadline)
	if err != nil {
		sn.tenantsErrors.Inc()
		return nil, err
	}
	readTraceFromResponse(qt, resp)
	tenants, _, err := readStringResponse(resp, "tenants")
	if err != nil {
		sn.tenantsErrors.Inc()
		return nil, fmt.Errorf("cannot get tenants from vmselect %s: %w", sn.baseURL, err)
	}
	return tenants, nil
}

// getSeriesCount returns the number of series from the HTTP select node.
func (sn *httpSelectNode) getSeriesCount(qt *querytracer.Tracer, accountID, projectID uint32, deadline searchutil.Deadline) (uint64, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.seriesCountRequests.Inc()

	// Body: uint32(accountID) + uint32(projectID) = 8 bytes
	var body []byte
	body = encoding.MarshalUint32(body, accountID)
	body = encoding.MarshalUint32(body, projectID)

	resp, err := sn.doPost(qt, "seriesCount", body, deadline)
	if err != nil {
		sn.seriesCountErrors.Inc()
		return 0, false, err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.seriesCountErrors.Inc()
		return 0, false, fmt.Errorf("unexpected HTTP status %d from seriesCount: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var result struct {
		SeriesCount uint64 `json:"seriesCount"`
		IsPartial   bool   `json:"isPartial"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sn.seriesCountErrors.Inc()
		return 0, false, fmt.Errorf("cannot decode seriesCount response from vmselect %s: %w", sn.baseURL, err)
	}
	return result.SeriesCount, result.IsPartial, nil
}

// getTSDBStatus returns TSDB status from the HTTP select node.
func (sn *httpSelectNode) getTSDBStatus(qt *querytracer.Tracer, requestData []byte, focusLabel string, topN int, deadline searchutil.Deadline) (*storage.TSDBStatus, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.tsdbStatusRequests.Inc()

	// Body: uint64(sqLen) + sqData + uint64(len(focusLabel)) + focusLabel + uint32(topN)
	focusLabelBytes := []byte(focusLabel)
	var body []byte
	body = encoding.MarshalUint64(body, uint64(len(requestData)))
	body = append(body, requestData...)
	body = encoding.MarshalUint64(body, uint64(len(focusLabelBytes)))
	body = append(body, focusLabelBytes...)
	body = append(body, encoding.MarshalUint32(nil, uint32(topN))...)

	resp, err := sn.doPost(qt, "tsdbStatus", body, deadline)
	if err != nil {
		sn.tsdbStatusErrors.Inc()
		return nil, false, err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.tsdbStatusErrors.Inc()
		return nil, false, fmt.Errorf("unexpected HTTP status %d from tsdbStatus: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var result struct {
		Status    *storage.TSDBStatus `json:"status"`
		IsPartial bool                `json:"isPartial"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sn.tsdbStatusErrors.Inc()
		return nil, false, fmt.Errorf("cannot decode tsdbStatus response from vmselect %s: %w", sn.baseURL, err)
	}
	return result.Status, result.IsPartial, nil
}

// getSearchMetricNames returns metric names from the HTTP select node.
func (sn *httpSelectNode) getSearchMetricNames(qt *querytracer.Tracer, requestData []byte, deadline searchutil.Deadline) ([]string, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.metricNamesRequests.Inc()

	resp, err := sn.doPost(qt, "searchMetricNames", requestData, deadline)
	if err != nil {
		sn.metricNamesErrors.Inc()
		return nil, false, err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.metricNamesErrors.Inc()
		return nil, false, fmt.Errorf("unexpected HTTP status %d from searchMetricNames: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var result struct {
		MetricNames []string `json:"metricNames"`
		IsPartial   bool     `json:"isPartial"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sn.metricNamesErrors.Inc()
		return nil, false, fmt.Errorf("cannot decode searchMetricNames response from vmselect %s: %w", sn.baseURL, err)
	}
	return result.MetricNames, result.IsPartial, nil
}

// deleteSeries deletes series matching the given requestData on the HTTP select node.
func (sn *httpSelectNode) deleteSeries(qt *querytracer.Tracer, requestData []byte, deadline searchutil.Deadline) (int, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.deleteRequests.Inc()

	resp, err := sn.doPost(qt, "deleteSeries", requestData, deadline)
	if err != nil {
		sn.deleteErrors.Inc()
		return 0, err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.deleteErrors.Inc()
		return 0, fmt.Errorf("unexpected HTTP status %d from deleteSeries: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var result struct {
		DeletedCount int `json:"deletedCount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		sn.deleteErrors.Inc()
		return 0, fmt.Errorf("cannot decode deleteSeries response from vmselect %s: %w", sn.baseURL, err)
	}
	return result.DeletedCount, nil
}

// registerMetricNames registers metric names on the HTTP select node.
func (sn *httpSelectNode) registerMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline searchutil.Deadline) error {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.registerRequests.Inc()

	// Body: uint64(count) + for each: uint64(len(metricNameRaw)) + metricNameRaw + int64(timestamp)
	var body []byte
	body = encoding.MarshalUint64(body, uint64(len(mrs)))
	for _, mr := range mrs {
		body = encoding.MarshalUint64(body, uint64(len(mr.MetricNameRaw)))
		body = append(body, mr.MetricNameRaw...)
		body = encoding.MarshalUint64(body, uint64(mr.Timestamp))
	}

	resp, err := sn.doPost(qt, "registerMetricNames", body, deadline)
	if err != nil {
		sn.registerErrors.Inc()
		return err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.registerErrors.Inc()
		return fmt.Errorf("unexpected HTTP status %d from registerMetricNames at vmselect %s: %s", resp.StatusCode, sn.baseURL, strings.TrimSpace(string(b)))
	}
	return nil
}

// getTagValueSuffixes returns tag value suffixes from the HTTP select node.
func (sn *httpSelectNode) getTagValueSuffixes(qt *querytracer.Tracer, accountID, projectID uint32, tr storage.TimeRange,
	tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline searchutil.Deadline,
) ([]string, bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.tagSuffixRequests.Inc()

	// Body: uint32(accountID) + uint32(projectID) + int64(min) + int64(max) +
	//       uint64(len(tagKey)) + tagKey + uint64(len(tagValuePrefix)) + tagValuePrefix +
	//       byte(delimiter) + uint32(maxSuffixes)
	var body []byte
	body = encoding.MarshalUint32(body, accountID)
	body = encoding.MarshalUint32(body, projectID)
	body = encoding.MarshalInt64(body, tr.MinTimestamp)
	body = encoding.MarshalInt64(body, tr.MaxTimestamp)
	tagKeyBytes := []byte(tagKey)
	body = encoding.MarshalUint64(body, uint64(len(tagKeyBytes)))
	body = append(body, tagKeyBytes...)
	tagValuePrefixBytes := []byte(tagValuePrefix)
	body = encoding.MarshalUint64(body, uint64(len(tagValuePrefixBytes)))
	body = append(body, tagValuePrefixBytes...)
	body = append(body, delimiter)
	body = append(body, encoding.MarshalUint32(nil, uint32(maxSuffixes))...)

	resp, err := sn.doPost(qt, "tagValueSuffixes", body, deadline)
	if err != nil {
		sn.tagSuffixErrors.Inc()
		return nil, false, err
	}
	readTraceFromResponse(qt, resp)
	suffixes, isPartial, err := readStringResponse(resp, "tagValueSuffixes")
	if err != nil {
		sn.tagSuffixErrors.Inc()
		return nil, false, fmt.Errorf("cannot get tag value suffixes from vmselect %s: %w", sn.baseURL, err)
	}
	return suffixes, isPartial, nil
}

// processSearchQuery performs a streaming search on the HTTP select node.
// The processBlock callback is called for each raw metric block received.
func (sn *httpSelectNode) processSearchQuery(qt *querytracer.Tracer, requestData []byte,
	processBlock func(rawBlock []byte, workerID uint) error, workerID uint, deadline searchutil.Deadline,
) (bool, error) {
	sn.concurrentQueries.Inc()
	defer sn.concurrentQueries.Dec()
	sn.searchRequests.Inc()

	resp, err := sn.doPost(qt, "search", requestData, deadline)
	if err != nil {
		sn.searchErrors.Inc()
		return false, err
	}
	defer resp.Body.Close()
	readTraceFromResponse(qt, resp)

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		sn.searchErrors.Inc()
		return false, fmt.Errorf("unexpected HTTP status %d from search at vmselect %s: %s",
			resp.StatusCode, sn.baseURL, strings.TrimSpace(string(b)))
	}

	isPartial := resp.Header.Get(httpHeaderPartialResponse) == "true"

	// Read binary streaming response: sequence of [uint64(size) + blockData] pairs.
	sizeBuf := make([]byte, 8)
	blocksRead := 0
	for {
		if _, err := io.ReadFull(resp.Body, sizeBuf); err != nil {
			sn.searchErrors.Inc()
			return false, fmt.Errorf("cannot read block size from search response at vmselect %s: %w", sn.baseURL, err)
		}
		blockSize := int(encoding.UnmarshalUint64(sizeBuf))
		if blockSize == 0 {
			// End-of-stream marker.
			break
		}
		if blockSize > maxMetricBlockSize {
			sn.searchErrors.Inc()
			return false, fmt.Errorf("too big block size %d from search response at vmselect %s; max allowed %d",
				blockSize, sn.baseURL, maxMetricBlockSize)
		}
		block := make([]byte, blockSize)
		if _, err := io.ReadFull(resp.Body, block); err != nil {
			sn.searchErrors.Inc()
			return false, fmt.Errorf("cannot read block #%d from search response at vmselect %s: %w",
				blocksRead, sn.baseURL, err)
		}
		blocksRead++
		sn.metricBlocksRead.Inc()
		if err := processBlock(block, workerID); err != nil {
			sn.searchErrors.Inc()
			return false, fmt.Errorf("cannot process block #%d from search response at vmselect %s: %w",
				blocksRead, sn.baseURL, err)
		}
	}
	return isPartial, nil
}

// readTraceFromResponse extracts the trace JSON from the response header and adds it to qt.
func readTraceFromResponse(qt *querytracer.Tracer, resp *http.Response) {
	traceJSON := resp.Header.Get(httpHeaderTraceJSON)
	if traceJSON == "" {
		return
	}
	if err := qt.AddJSON([]byte(traceJSON)); err != nil {
		logger.Errorf("cannot parse trace JSON from vmselect response: %s", err)
	}
}

// getMetricsMetadata returns metrics metadata from the HTTP select node.
// This is a stub that returns an empty result since metadata is not critical for the HTTP path.
func (sn *httpSelectNode) getMetricsMetadata(_ *querytracer.Tracer, _ *storage.TenantToken, _ int, _ string, _ searchutil.Deadline) ([]*metricsmetadata.Row, bool, error) {
	return nil, false, nil
}
