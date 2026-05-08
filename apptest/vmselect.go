package apptest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"testing"
)

// Vmselect holds the state of a vmselect app and provides vmselect-specific
// functions.
type Vmselect struct {
	*app
	*ServesMetrics

	httpListenAddr          string
	clusternativeListenAddr string
	cli                     *Client
}

// StartVmselect starts an instance of vmselect with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmselect(instance string, flags []string, cli *Client, output io.Writer) (*Vmselect, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/vmselect-race", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-clusternativeListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
			vmselectAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return &Vmselect{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
		cli:                     cli,
	}, nil
}

// ClusternativeListenAddr returns the address at which the vmselect process is
// listening for connections from other vmselect apps.
func (app *Vmselect) ClusternativeListenAddr() string {
	return app.clusternativeListenAddr
}

// HTTPAddr returns the address at which the vmselect process is
// listening for incoming HTTP requests.
func (app *Vmselect) HTTPAddr() string {
	return app.httpListenAddr
}

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a request to
// /prometheus/api/v1/export endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export
func (app *Vmselect) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/export", opts)
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1ExportNative is a test helper function that performs the export of
// raw samples in native binary format by sending a request to
// /prometheus/api/v1/export/native endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportnative
func (app *Vmselect) PrometheusAPIV1ExportNative(t *testing.T, query string, opts QueryOpts) []byte {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/export/native", opts)
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return []byte(res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a request to /prometheus/api/v1/query endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query
func (app *Vmselect) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/query", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a request to
// /prometheus/api/v1/query_range endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query_range
func (app *Vmselect) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/query_range", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Series retrieves list of time series that match the query by
// sending a request to /prometheus/api/v1/series endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (app *Vmselect) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/series", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// PrometheusAPIV1SeriesCount retrieves the total number of time series by
// sending a request to /prometheus/api/v1/series/count endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (app *Vmselect) PrometheusAPIV1SeriesCount(t *testing.T, opts QueryOpts) *PrometheusAPIV1SeriesCountResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/series/count", opts)
	values := opts.asURLValues()
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1SeriesCountResponse(t, res)
}

// PrometheusAPIV1Labels retrieves the label names for time series that match a
// query by sending a request to /prometheus/api/v1/labels endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1labels
func (app *Vmselect) PrometheusAPIV1Labels(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1LabelsResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/labels", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1LabelsResponse(t, res)
}

// PrometheusAPIV1LabelValues retrieves the labels values for the metrics that
// match the query by sending a request to /prometheus/api/v1/label/.../values
// endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1labelvalues
func (app *Vmselect) PrometheusAPIV1LabelValues(t *testing.T, labelName, matchQuery string, opts QueryOpts) *PrometheusAPIV1LabelValuesResponse {
	t.Helper()

	suffix := fmt.Sprintf("prometheus/api/v1/label/%s/values", labelName)
	url := getClusterPath(app.httpListenAddr, "select", suffix, opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1LabelValuesResponse(t, res)
}

// PrometheusAPIV1Metadata retrieves metadata for the given metric by sending a
// request to /prometheus/api/v1/metadata endpoint.
func (app *Vmselect) PrometheusAPIV1Metadata(t *testing.T, metric string, limit int, opts QueryOpts) *PrometheusAPIV1Metadata {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/metadata", opts)
	values := opts.asURLValues()
	values.Add("metric", metric)
	values.Add("limit", strconv.Itoa(limit))
	res, _ := app.cli.PostForm(t, url, values, opts.Headers)
	return NewPrometheusAPIV1Metadata(t, res)
}

// PrometheusAPIV1AdminTSDBDeleteSeries deletes the series that match the query
// by sending a request to /prometheus/api/v1/admin/tsdb/delete_series.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series
func (app *Vmselect) PrometheusAPIV1AdminTSDBDeleteSeries(t *testing.T, matchQuery string, opts QueryOpts) {
	t.Helper()

	queryURL := getClusterPath(app.httpListenAddr, "delete", "prometheus/api/v1/admin/tsdb/delete_series", opts)
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)
	res, statusCode := app.cli.PostForm(t, queryURL, values, opts.Headers)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// PrometheusAPIV1StatusMetricNamesStats sends a query to
// /prometheus/api/v1/status/metric_names_stats endpoint and returns the metric
// usage stats response for given params.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (app *Vmselect) PrometheusAPIV1StatusMetricNamesStats(t *testing.T, limit, le, matchPattern string, opts QueryOpts) MetricNamesStatsResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("limit", limit)
	values.Add("le", le)
	values.Add("match_pattern", matchPattern)
	queryURL := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/status/metric_names_stats", opts)

	res, statusCode := app.cli.PostForm(t, queryURL, values, opts.Headers)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}
	var resp MetricNamesStatsResponse
	if err := json.Unmarshal([]byte(res), &resp); err != nil {
		t.Fatalf("could not unmarshal metric names stats response data:\n%s\n err: %v", res, err)
	}
	return resp
}

// PrometheusAPIV1AdminStatusMetricNamesStatsReset resets the metric name usage
// stats by sending a request to
// /prometheus/api/v1/admin/status/metric_names_stats/reset endpoint
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (app *Vmselect) PrometheusAPIV1AdminStatusMetricNamesStatsReset(t *testing.T, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/admin/api/v1/admin/status/metric_names_stats/reset", app.httpListenAddr)
	values := opts.asURLValues()

	res, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// PrometheusAPIV1StatusTSDB retrieves the TSDB status for the time series that
// match the query on the given date by sending a request to
// /prometheus/api/v1/status/tsdb endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#tsdb-stats
func (app *Vmselect) PrometheusAPIV1StatusTSDB(t *testing.T, matchQuery string, date string, topN string, opts QueryOpts) TSDBStatusResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "prometheus/api/v1/status/tsdb", opts)
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

	res, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
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
func (app *Vmselect) GraphiteMetricsIndex(t *testing.T, opts QueryOpts) GraphiteMetricsIndexResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/metrics/index.json", opts)
	res, statusCode := app.cli.Get(t, url, opts.Headers)
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
func (app *Vmselect) GraphiteMetricsFind(t *testing.T, query string, opts QueryOpts) GraphiteMetricsFindResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/metrics/find", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	resText, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
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
func (app *Vmselect) GraphiteMetricsExpand(t *testing.T, query string, opts QueryOpts) GraphiteMetricsExpandResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/metrics/expand", opts)
	values := opts.asURLValues()
	values.Add("query", query)
	resText, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
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
func (app *Vmselect) GraphiteRender(t *testing.T, target string, opts QueryOpts) GraphiteRenderResponse {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/render", opts)
	values := opts.asURLValues()
	values.Add("format", "json")
	values.Add("target", target)
	resText, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
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
func (app *Vmselect) GraphiteTagsTagSeries(t *testing.T, record string, opts QueryOpts) {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/tags/tagSeries", opts)
	values := opts.asURLValues()
	values.Add("path", record)
	_, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
	if got, want := statusCode, http.StatusNotImplemented; got != want {
		t.Fatalf("unexpected status code: got %d, want %d", got, want)
	}
}

// GraphiteTagsTagMultiSeries is a test helper function that registers Graphite
// tags for a multiple time series by sending a request to
// /graphite/tags/tagSeries endpoint.
func (app *Vmselect) GraphiteTagsTagMultiSeries(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := getClusterPath(app.httpListenAddr, "select", "graphite/tags/tagMultiSeries", opts)
	values := opts.asURLValues()
	for _, rec := range records {
		values.Add("path", rec)
	}
	_, statusCode := app.cli.PostForm(t, url, values, opts.Headers)
	if got, want := statusCode, http.StatusNotImplemented; got != want {
		t.Fatalf("unexpected status code: got %d, want %d", got, want)
	}
}

// APIV1AdminTenants sends a query to a /admin/tenants endpoint
func (app *Vmselect) APIV1AdminTenants(t *testing.T) *AdminTenantsResponse {
	t.Helper()

	tenantsURL := fmt.Sprintf("http://%s/admin/tenants", app.httpListenAddr)
	res, statusCode := app.cli.Get(t, tenantsURL, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	tenants := &AdminTenantsResponse{}
	if err := json.Unmarshal([]byte(res), tenants); err != nil {
		t.Fatalf("could not unmarshal tenants response data:\n%s\n err: %v", res, err)
	}

	return tenants
}

// String returns the string representation of the vmselect app state.
func (app *Vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
