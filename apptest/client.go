package apptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	otlppb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/golang/snappy"
)

// Client is used for interacting with the apps over the network.
type Client struct {
	httpCli *http.Client
}

// NewClient creates a new client.
func NewClient() *Client {
	return &Client{
		httpCli: &http.Client{
			Transport: httputil.NewTransport(false, "apptest_client"),
		},
	}
}

// CloseConnections closes client connections.
func (c *Client) CloseConnections() {
	c.httpCli.CloseIdleConnections()
}

// Get sends an HTTP GET request, returns
// the response body and status code to the caller.
func (c *Client) Get(t *testing.T, url string, headers http.Header) (string, int) {
	t.Helper()
	return c.do(t, http.MethodGet, url, nil, headers)
}

// Post sends an HTTP POST request, returns
// the response body and status code to the caller.
func (c *Client) Post(t *testing.T, url string, data []byte, headers http.Header) (string, int) {
	t.Helper()
	return c.do(t, http.MethodPost, url, data, headers)
}

// PostForm sends an HTTP POST request containing the POST-form data with attached getHeaders, returns
// the response body and status code to the caller.
func (c *Client) PostForm(t *testing.T, url string, data url.Values, headers http.Header) (string, int) {
	t.Helper()
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.Post(t, url, []byte(data.Encode()), headers)
}

// Delete sends an HTTP DELETE request and returns the response body and status code
// to the caller.
func (c *Client) Delete(t *testing.T, url string) (string, int) {
	t.Helper()
	return c.do(t, http.MethodDelete, url, nil, nil)
}

// do prepares an HTTP request, sends it to the server, receives the response
// from the server, returns the response body and status code to the caller.
func (c *Client) do(t *testing.T, method, url string, data []byte, headers http.Header) (string, int) {
	t.Helper()

	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("could not create a HTTP request: %v", err)
	}

	req.Header = headers
	res, err := c.httpCli.Do(req)
	if err != nil {
		t.Fatalf("could not send HTTP request: %v", err)
	}

	body := readAllAndClose(t, res.Body)

	return body, res.StatusCode
}

func (c *Client) Write(t *testing.T, address string, data []string) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("cannot dial %s: %s", address, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	d := []byte(strings.Join(data, "\n"))
	n, err := conn.Write(d)
	if err != nil {
		t.Fatalf("cannot write %d bytes to %s: %s", len(d), address, err)
	}
	if n != len(d) {
		t.Fatalf("BUG: conn.Write() returned unexpected number of written bytes to %s; got %d; want %d", address, n, len(d))
	}
}

// getClusterPath returns path in cluster's URL format.
// Based on QueryOpts, it will either put tenant ID into URL
// or will skip it if tenant is set via HTTP headers.
func getClusterPath(addr, prefix, suffix string, o QueryOpts) string {
	if o.Tenant != "" {
		// QueryOpts.Tenant has priority over headers
		return tenantViaURL(addr, prefix, o.Tenant, suffix)
	}

	h := o.getHeaders()
	if h.Get("AccountID") != "" || h.Get("ProjectID") != "" {
		return tenantViaHeaders(addr, prefix, suffix)
	}

	// tenant is missing in QueryOpts and in HTTP headers. Falling back to default 0:0 tenant in URL
	return tenantViaURL(addr, prefix, "0:0", suffix)
}

// tenantViaURL returns path in cluster's URL format with tenant specified in URL
func tenantViaURL(addr, prefix, tenant, suffix string) string {
	return fmt.Sprintf("http://%s/%s/%s/%s", addr, prefix, tenant, suffix)
}

// tenantViaHeaders returns path in cluster's URL format where tenant is omitted in URL
// Only supported if -enableMultitenancyViaHeaders is specified
func tenantViaHeaders(addr, prefix, suffix string) string {
	return fmt.Sprintf("http://%s/%s/%s", addr, prefix, suffix)
}

// readAllAndClose reads everything from the response body and then closes it.
func readAllAndClose(t *testing.T, responseBody io.ReadCloser) string {
	t.Helper()

	defer responseBody.Close()
	b, err := io.ReadAll(responseBody)
	if err != nil {
		t.Fatalf("could not read response body: %d", err)
	}
	return string(b)
}

// metricsClient is used to retrieve the app's metrics.
//
// This type is expected to be embedded by the apps that serve metrics.
type metricsClient struct {
	cli *Client
	url string
}

func newMetricsClient(cli *Client, addr string) *metricsClient {
	return &metricsClient{
		cli: cli,
		url: fmt.Sprintf("http://%s/metrics", addr),
	}
}

// GetIntMetric retrieves the value of a metric served by an app at /metrics URL.
// The value is then converted to int.
func (c *metricsClient) GetIntMetric(t *testing.T, metricName string) int {
	t.Helper()

	return int(c.GetMetric(t, metricName))
}

// GetMetric retrieves the value of a metric served by an app at /metrics URL.
func (c *metricsClient) GetMetric(t *testing.T, metricName string) float64 {
	t.Helper()

	metrics, statusCode := c.cli.Get(t, c.url, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
	for _, metric := range strings.Split(metrics, "\n") {
		value, found := strings.CutPrefix(metric, metricName)
		if found {
			value = strings.Trim(value, " ")
			res, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatalf("could not parse metric value %s: %v", metric, err)
			}
			return res
		}
	}
	t.Fatalf("metric not found: %s", metricName)
	return 0
}

// GetMetricsByPrefix retrieves the values of all metrics that start with given
// prefix.
func (c *metricsClient) GetMetricsByPrefix(t *testing.T, prefix string) []float64 {
	t.Helper()

	values := []float64{}

	metrics, statusCode := c.cli.Get(t, c.url, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
	for _, metric := range strings.Split(metrics, "\n") {
		if !strings.HasPrefix(metric, prefix) {
			continue
		}

		parts := strings.Split(metric, " ")
		if len(parts) < 2 {
			t.Fatalf("unexpected record format: got %q, want metric name and value separated by a space", metric)
		}

		value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			t.Fatalf("could not parse metric value %s: %v", metric, err)
		}

		values = append(values, value)
	}
	return values
}

func (c *metricsClient) GetMetricsByRegexp(t *testing.T, re *regexp.Regexp) []float64 {
	t.Helper()

	values := []float64{}

	metrics, statusCode := c.cli.Get(t, c.url, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
	for _, metric := range strings.Split(metrics, "\n") {
		if !re.MatchString(metric) {
			continue
		}

		parts := strings.Split(metric, " ")
		if len(parts) < 2 {
			t.Fatalf("unexpected record format: got %q, want metric name and value separated by a space", metric)
		}

		value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			t.Fatalf("could not parse metric value %s: %v", metric, err)
		}

		values = append(values, value)
	}
	return values
}

// rpcRowsSentTotal retrieves the values of all vminsert
// `vm_rpc_rows_sent_total` metrics (there will be one for each vmstorage) and
// returns their integer sum.
func (c *metricsClient) rpcRowsSentTotal(t *testing.T) int {
	total := 0.0
	for _, v := range c.GetMetricsByPrefix(t, "vm_rpc_rows_sent_total") {
		total += v
	}
	return int(total)
}

type vmselectClient struct {
	cli                      *Client
	url                      func(op, path string, opts QueryOpts) string
	metricNamesStatsResetURL string
	tenantsURL               string
}

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a request to
// /prometheus/api/v1/export endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export
func (c *vmselectClient) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/export", opts)
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1ExportNative is a test helper function that performs the export of
// raw samples in native binary format by sending a request to
// /prometheus/api/v1/export/native endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportnative
func (c *vmselectClient) PrometheusAPIV1ExportNative(t *testing.T, query string, opts QueryOpts) []byte {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/export/native", opts)
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return []byte(res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a request to /prometheus/api/v1/query endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query
func (c *vmselectClient) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/query", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a request to
// /prometheus/api/v1/query_range endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query_range
func (c *vmselectClient) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/query_range", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Series retrieves list of time series that match the query by
// sending a request to /prometheus/api/v1/series endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (c *vmselectClient) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/series", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// PrometheusAPIV1SeriesCount retrieves the total number of time series by
// sending a request to /prometheus/api/v1/series/count endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (c *vmselectClient) PrometheusAPIV1SeriesCount(t *testing.T, opts QueryOpts) *PrometheusAPIV1SeriesCountResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/series/count", opts)
	values := opts.asURLValues()
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1SeriesCountResponse(t, res)
}

// PrometheusAPIV1Labels retrieves the label names for time series that match a
// query by sending a request to /prometheus/api/v1/labels endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1labels
func (c *vmselectClient) PrometheusAPIV1Labels(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1LabelsResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/labels", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1LabelsResponse(t, res)
}

// PrometheusAPIV1LabelValues retrieves the labels values for the metrics that
// match the query by sending a request to /prometheus/api/v1/label/.../values
// endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1labelvalues
func (c *vmselectClient) PrometheusAPIV1LabelValues(t *testing.T, labelName, matchQuery string, opts QueryOpts) *PrometheusAPIV1LabelValuesResponse {
	t.Helper()
	path := fmt.Sprintf("prometheus/api/v1/label/%s/values", labelName)
	url := c.url("select", path, opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1LabelValuesResponse(t, res)
}

// PrometheusAPIV1Metadata retrieves metadata for the given metric by sending a
// request to /prometheus/api/v1/metadata endpoint.
func (c *vmselectClient) PrometheusAPIV1Metadata(t *testing.T, metric string, limit int, opts QueryOpts) *PrometheusAPIV1Metadata {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/metadata", opts)
	values := opts.asURLValues()
	values.Add("metric", metric)
	values.Add("limit", strconv.Itoa(limit))
	res, _ := c.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1Metadata(t, res)
}

// PrometheusAPIV1AdminTSDBDeleteSeries deletes the series that match the query
// by sending a request to /prometheus/api/v1/admin/tsdb/delete_series.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series
func (c *vmselectClient) PrometheusAPIV1AdminTSDBDeleteSeries(t *testing.T, matchQuery string, opts QueryOpts) {
	t.Helper()

	url := c.url("delete", "prometheus/api/v1/admin/tsdb/delete_series", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// PrometheusAPIV1StatusMetricNamesStats sends a query to
// /prometheus/api/v1/status/metric_names_stats endpoint and returns the metric
// usage stats response for given params.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (c *vmselectClient) PrometheusAPIV1StatusMetricNamesStats(t *testing.T, limit, le, matchPattern string, opts QueryOpts) MetricNamesStatsResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/status/metric_names_stats", opts)
	values := opts.asURLValues()
	values.Add("limit", limit)
	values.Add("le", le)
	values.Add("match_pattern", matchPattern)
	res, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}
	var resp MetricNamesStatsResponse
	if err := json.Unmarshal([]byte(res), &resp); err != nil {
		t.Fatalf("could not unmarshal metric names stats response data:\n%s\n err: %v", res, err)
	}
	return resp
}

// PrometheusAPIV1StatusTSDB retrieves the TSDB status for the time series that
// match the query on the given date by sending a request to
// /prometheus/api/v1/status/tsdb endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#tsdb-stats
func (c *vmselectClient) PrometheusAPIV1StatusTSDB(t *testing.T, matchQuery string, date string, topN string, opts QueryOpts) TSDBStatusResponse {
	t.Helper()
	url := c.url("select", "prometheus/api/v1/status/tsdb", opts)
	values := opts.asURLValues()
	addNonEmpty := func(name, value string) {
		if len(value) == 0 {
			return
		}
		values.Add(name, value)
	}
	addNonEmpty("match[]", matchQuery)
	addNonEmpty("topN", topN)
	addNonEmpty("date", date)
	res, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var status TSDBStatusResponse
	if err := json.Unmarshal([]byte(res), &status); err != nil {
		t.Fatalf("could not unmarshal tsdb status response data:\n%s\n err: %v", res, err)
	}
	status.Sort()
	return status
}

// GraphiteMetricsIndex retrieves the list of all metrics by sending a request
// to /graphite/metrics/index.json endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#metrics-api
func (c *vmselectClient) GraphiteMetricsIndex(t *testing.T, opts QueryOpts) GraphiteMetricsIndexResponse {
	t.Helper()

	url := c.url("select", "graphite/metrics/index.json", opts)
	res, statusCode := c.cli.Get(t, url, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var index GraphiteMetricsIndexResponse
	if err := json.Unmarshal([]byte(res), &index); err != nil {
		t.Fatalf("could not unmarshal metrics index response data:\n%s\n err: %v", res, err)
	}
	return index
}

// GraphiteMetricsFind finds metrics under a given path by sending a request
// to /metrics/find endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#metrics-api
// and https://graphite.readthedocs.io/en/latest/metrics_api.html#metrics-find
func (c *vmselectClient) GraphiteMetricsFind(t *testing.T, query string, opts QueryOpts) GraphiteMetricsFindResponse {
	t.Helper()

	url := c.url("select", "graphite/metrics/find", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	resText, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, resText)
	}

	var res GraphiteMetricsFindResponse
	if err := json.Unmarshal([]byte(resText), &res); err != nil {
		t.Fatalf("could not unmarshal response data:\n%s\n err: %v", resText, err)
	}
	return res
}

// GraphiteMetricsExpand expands the given query with matching paths by sending
// a request to /graphite/metrics/expand endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#metrics-api
// and https://graphite.readthedocs.io/en/latest/metrics_api.html#metrics-expand
func (c *vmselectClient) GraphiteMetricsExpand(t *testing.T, query string, opts QueryOpts) GraphiteMetricsExpandResponse {
	t.Helper()

	url := c.url("select", "graphite/metrics/expand", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	resText, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, resText)
	}

	var res GraphiteMetricsExpandResponse
	if err := json.Unmarshal([]byte(resText), &res); err != nil {
		t.Fatalf("could not unmarshal response data:\n%s\n err: %v", resText, err)
	}
	return res
}

// GraphiteRender retrieves the raw metric data by sending a request to
// /graphite/render endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#render-api
// and https://graphite-api.readthedocs.io/en/latest/api.html#the-render-api-render
func (c *vmselectClient) GraphiteRender(t *testing.T, target string, opts QueryOpts) GraphiteRenderResponse {
	t.Helper()

	url := c.url("select", "graphite/render", opts)
	values := opts.asURLValues()
	values.Add("format", "json")
	values.Add("target", target)
	resText, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, resText)
	}

	var res GraphiteRenderResponse
	if err := json.Unmarshal([]byte(resText), &res); err != nil {
		t.Fatalf("could not unmarshal response data:\n%s\n err: %v", resText, err)
	}
	return res
}

// GraphiteTagsTagSeries is a test helper function that registers Graphite tags
// for a single time series by sending a request to /graphite/tags/tagSeries
// endpoint.
func (c *vmselectClient) GraphiteTagsTagSeries(t *testing.T, record string, opts QueryOpts) {
	t.Helper()

	url := c.url("select", "graphite/tags/tagSeries", opts)
	values := opts.asURLValues()
	values.Add("path", record)
	_, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if got, want := statusCode, http.StatusNotImplemented; got != want {
		t.Fatalf("unexpected status code: got %d, want %d", got, want)
	}
}

// GraphiteTagsTagMultiSeries is a test helper function that registers Graphite
// tags for a multiple time series by sending a request to
// /graphite/tags/tagSeries endpoint.
func (c *vmselectClient) GraphiteTagsTagMultiSeries(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.url("select", "graphite/tags/tagMultiSeries", opts)
	values := opts.asURLValues()
	for _, rec := range records {
		values.Add("path", rec)
	}
	_, statusCode := c.cli.PostForm(t, url, values, opts.Headers)
	if got, want := statusCode, http.StatusNotImplemented; got != want {
		t.Fatalf("unexpected status code: got %d, want %d", got, want)
	}
}

// PrometheusAPIV1AdminStatusMetricNamesStatsReset resets the metric name usage
// stats by sending a request to
// /prometheus/api/v1/admin/status/metric_names_stats/reset endpoint
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (c *vmselectClient) PrometheusAPIV1AdminStatusMetricNamesStatsReset(t *testing.T, opts QueryOpts) {
	t.Helper()
	values := opts.asURLValues()
	res, statusCode := c.cli.PostForm(t, c.metricNamesStatsResetURL, values, opts.Headers)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// APIV1AdminTenants retrieves the list of tenants by sending a request to
// /admin/tenants endpoint.
func (c *vmselectClient) APIV1AdminTenants(t *testing.T, opts QueryOpts) *AdminTenantsResponse {
	t.Helper()
	res, statusCode := c.cli.Get(t, c.tenantsURL, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	tenants := &AdminTenantsResponse{}
	if err := json.Unmarshal([]byte(res), tenants); err != nil {
		t.Fatalf("could not unmarshal tenants response data:\n%s\n err: %v", res, err)
	}

	return tenants
}

type vminsertClient struct {
	cli                *Client
	url                func(op, path string, opts QueryOpts) string
	openTSDBURL        func(op, path string, opts QueryOpts) string
	graphiteListenAddr string
	sendBlocking       func(t *testing.T, numRecordsToSend int, send func())
}

// PrometheusAPIV1ImportCSV is a test helper function that inserts a collection
// of records in CSV format for the given tenant by sending an HTTP POST
// request to prometheus/api/v1/import/csv vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (c *vminsertClient) PrometheusAPIV1ImportCSV(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "prometheus/api/v1/import/csv", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	headers := opts.getHeaders()
	headers.Set("Content-Type", "text/plain")
	c.sendBlocking(t, len(records), func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1ImportNative is a test helper function that inserts a collection
// of records in Native format for the given tenant by sending an HTTP POST
// request to prometheus/api/v1/import/native vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (c *vminsertClient) PrometheusAPIV1ImportNative(t *testing.T, data []byte, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "prometheus/api/v1/import/native", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	headers := opts.getHeaders()
	headers.Set("Content-Type", "text/plain")
	c.sendBlocking(t, 1, func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1Write is a test helper function that inserts a
// collection of records in Prometheus remote-write format by sending a HTTP
// POST request to /prometheus/api/v1/write vminsert endpoint.
func (c *vminsertClient) PrometheusAPIV1Write(t *testing.T, wr prompb.WriteRequest, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "prometheus/api/v1/write", opts)
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	recordsCount := len(wr.Timeseries)
	if prommetadata.IsEnabled() {
		recordsCount += len(wr.Metadata)
	}
	headers := opts.getHeaders()
	headers.Set("Content-Type", "application/x-protobuf")
	c.sendBlocking(t, recordsCount, func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to
// /prometheus/api/v1/import/prometheus vminsert endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1importprometheus
func (c *vminsertClient) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "prometheus/api/v1/import/prometheus", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	var recordsCount int
	var metadataRecords int
	uniqueMetadataMetricNames := make(map[string]struct{})
	for _, record := range records {
		// metric metadata has the following format:
		//# HELP importprometheus_series
		//# TYPE importprometheus_series
		// it results into single metadata record
		if strings.HasPrefix(record, "# ") {
			metadataItems := strings.Split(record, " ")
			if len(metadataItems) < 3 {
				t.Fatalf("BUG: unexpected metadata format=%q", record)
			}
			metricName := metadataItems[2]
			if _, ok := uniqueMetadataMetricNames[metricName]; ok {
				continue
			}
			uniqueMetadataMetricNames[metricName] = struct{}{}
			metadataRecords++
			continue
		}
		recordsCount++
	}
	if prommetadata.IsEnabled() {
		recordsCount += metadataRecords
	}
	headers := opts.getHeaders()
	headers.Set("Content-Type", "text/plain")
	c.sendBlocking(t, recordsCount, func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// InfluxWrite is a test helper function that inserts a collection of records in
// Influx line format by sending a HTTP POST request to /influx/write endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#influxwrite
func (c *vminsertClient) InfluxWrite(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "influx/write", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}

	data := []byte(strings.Join(records, "\n"))
	headers := opts.getHeaders()
	headers.Set("Content-Type", "text/plain")
	c.sendBlocking(t, len(records), func() {
		t.Helper()
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// OpentelemetryV1Metrics is a test helper function that inserts a
// collection of records in Opentelemetry protocol format by sending a HTTP
// POST request to /opentelemetry/v1/metrics vminsert endpoint.
func (c *vminsertClient) OpentelemetryV1Metrics(t *testing.T, md otlppb.MetricsData, opts QueryOpts) {
	t.Helper()

	var recordsCount int
	for _, rss := range md.ResourceMetrics {
		for _, sm := range rss.ScopeMetrics {
			recordsCount += len(sm.Metrics)
			for _, m := range sm.Metrics {
				if prommetadata.IsEnabled() {
					recordsCount += len(m.Metadata)
				}
			}
		}
	}
	url := c.url("insert", "opentelemetry/v1/metrics", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := md.MarshalProtobuf(nil)
	headers := opts.getHeaders()
	headers.Set("Content-Type", "application/x-protobuf")
	c.sendBlocking(t, recordsCount, func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusOK {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
		}
	})
}

// OpenTSDBAPIPut is a test helper function that inserts a collection of
// records in OpenTSDB format for the given tenant by sending an HTTP POST
// request to /opentsdb/api/put vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (c *vminsertClient) OpenTSDBAPIPut(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.openTSDBURL("insert", "opentsdb/api/put", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte("[" + strings.Join(records, ",") + "]")
	headers := opts.getHeaders()
	headers.Set("Content-Type", "application/json")
	c.sendBlocking(t, len(records), func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// ZabbixConnectorHistory is a test helper function that inserts a
// collection of records in zabbixconnector  format by sending a HTTP
// POST request to /zabbixconnector/api/v1/history vmsingle endpoint.
func (c *vminsertClient) ZabbixConnectorHistory(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := c.url("insert", "zabbixconnector/api/v1/history", opts)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	headers := opts.getHeaders()
	headers.Set("Content-Type", "application/json")
	c.sendBlocking(t, len(records), func() {
		_, statusCode := c.cli.Post(t, url, data, headers)
		if statusCode != http.StatusOK {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
		}
	})

}

// GraphiteWrite is a test helper function that sends a
// collection of records to graphiteListenAddr port.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting
func (c *vminsertClient) GraphiteWrite(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()
	c.cli.Write(t, c.graphiteListenAddr, records)
}

type vmstorageClient struct {
	cli            *Client
	httpListenAddr string
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (c *vmstorageClient) ForceFlush(t *testing.T) {
	t.Helper()

	url := fmt.Sprintf("http://%s/internal/force_flush", c.httpListenAddr)
	_, statusCode := c.cli.Get(t, url, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (c *vmstorageClient) ForceMerge(t *testing.T) {
	t.Helper()

	url := fmt.Sprintf("http://%s/internal/force_merge", c.httpListenAddr)
	_, statusCode := c.cli.Get(t, url, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// SnapshotCreate creates a database snapshot by sending a query to the
// /snapshot/create endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (c *vmstorageClient) SnapshotCreate(t *testing.T) *SnapshotCreateResponse {
	t.Helper()

	data, statusCode := c.cli.Post(t, c.SnapshotCreateURL(), nil, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotCreateResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotCreateURL returns the URL for creating snapshots.
func (c *vmstorageClient) SnapshotCreateURL() string {
	return fmt.Sprintf("http://%s/snapshot/create", c.httpListenAddr)
}

// APIV1AdminTSDBSnapshot creates a database snapshot by sending a query to the
// /api/v1/admin/tsdb/snapshot endpoint.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot.
func (c *vmstorageClient) APIV1AdminTSDBSnapshot(t *testing.T) *APIV1AdminTSDBSnapshotResponse {
	t.Helper()

	url := fmt.Sprintf("http://%s/api/v1/admin/tsdb/snapshot", c.httpListenAddr)
	data, statusCode := c.cli.Post(t, url, nil, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res APIV1AdminTSDBSnapshotResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal prometheus snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotList lists existing database snapshots by sending a query to the
// /snapshot/list endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (c *vmstorageClient) SnapshotList(t *testing.T) *SnapshotListResponse {
	t.Helper()

	url := fmt.Sprintf("http://%s/snapshot/list", c.httpListenAddr)
	data, statusCode := c.cli.Get(t, url, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotListResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot list response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDelete deletes a snapshot by sending a query to the
// /snapshot/delete endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (c *vmstorageClient) SnapshotDelete(t *testing.T, snapshotName string) *SnapshotDeleteResponse {
	t.Helper()

	url := fmt.Sprintf("http://%s/snapshot/delete?snapshot=%s", c.httpListenAddr, snapshotName)
	data, statusCode := c.cli.Delete(t, url)
	wantStatusCodes := map[int]bool{
		http.StatusOK:                  true,
		http.StatusInternalServerError: true,
	}
	if !wantStatusCodes[statusCode] {
		t.Fatalf("unexpected status code: got %d, want %v, resp text=%q", statusCode, wantStatusCodes, data)
	}

	var res SnapshotDeleteResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDeleteAll deletes all snapshots by sending a query to the
// /snapshot/delete_all endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (c *vmstorageClient) SnapshotDeleteAll(t *testing.T) *SnapshotDeleteAllResponse {
	t.Helper()

	url := fmt.Sprintf("http://%s/snapshot/delete_all", c.httpListenAddr)
	data, statusCode := c.cli.Post(t, url, nil, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotDeleteAllResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete all response: data=%q, err: %v", data, err)
	}

	return &res
}
