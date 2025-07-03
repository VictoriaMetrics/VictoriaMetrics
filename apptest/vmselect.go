package apptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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
func StartVmselect(instance string, flags []string, cli *Client) (*Vmselect, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/vmselect", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-clusternativeListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
			vmselectAddrRE,
		},
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
// raw samples in JSON line format by sending a HTTP POST request to
// /prometheus/api/v1/export vmselect endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export
func (app *Vmselect) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	exportURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/export", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := app.cli.PostForm(t, exportURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1ExportNative is a test helper function that performs the export of
// raw samples in native binary format by sending an HTTP POST request to
// /prometheus/api/v1/export/native vmselect endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportnative
func (app *Vmselect) PrometheusAPIV1ExportNative(t *testing.T, query string, opts QueryOpts) []byte {
	t.Helper()

	exportURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/export/native", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res, _ := app.cli.PostForm(t, exportURL, values)
	return []byte(res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a HTTP POST request to /prometheus/api/v1/query
// vmselect endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query
func (app *Vmselect) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/query", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("query", query)

	res, _ := app.cli.PostForm(t, queryURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a HTTP POST request to
// /prometheus/api/v1/query_range vmselect endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query_range
func (app *Vmselect) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/query_range", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("query", query)

	res, _ := app.cli.PostForm(t, queryURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Series sends a query to a /prometheus/api/v1/series endpoint
// and returns the list of time series that match the query.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (app *Vmselect) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/series", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	res, _ := app.cli.PostForm(t, seriesURL, values)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// DeleteSeries sends a query to a /prometheus/api/v1/admin/tsdb/delete_series
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1admintsdbdelete_series
func (app *Vmselect) DeleteSeries(t *testing.T, matchQuery string, opts QueryOpts) {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/delete/%s/prometheus/api/v1/admin/tsdb/delete_series", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	// TODO(@rtm0): Add DeleteSeriesResponse.
	res, _ := app.cli.PostForm(t, seriesURL, values)
	if res != "" {
		t.Fatalf("unexpected non-empty DeleteSeries response=%q", res)
	}
}

// MetricNamesStats sends a query to a /select/tenant/prometheus/api/v1/status/metric_names_stats endpoint
// and returns the statistics response for given params.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (app *Vmselect) MetricNamesStats(t *testing.T, limit, le, matchPattern string, opts QueryOpts) MetricNamesStatsResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("limit", limit)
	values.Add("le", le)
	values.Add("match_pattern", matchPattern)
	queryURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/status/metric_names_stats", app.httpListenAddr, opts.getTenant())

	res, statusCode := app.cli.PostForm(t, queryURL, values)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}
	var resp MetricNamesStatsResponse
	if err := json.Unmarshal([]byte(res), &resp); err != nil {
		t.Fatalf("could not unmarshal series response data:\n%s\n err: %v", res, err)
	}
	return resp
}

// MetricNamesStatsReset sends a query to a /admin/api/v1/status/metric_names_stats/reset endpoint
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
func (app *Vmselect) MetricNamesStatsReset(t *testing.T, opts QueryOpts) {
	t.Helper()

	values := opts.asURLValues()
	queryURL := fmt.Sprintf("http://%s/admin/api/v1/admin/status/metric_names_stats/reset", app.httpListenAddr)

	res, statusCode := app.cli.PostForm(t, queryURL, values)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// APIV1StatusTSDB sends a query to a /prometheus/api/v1/status/tsdb
// //
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#tsdb-stats
func (app *Vmselect) APIV1StatusTSDB(t *testing.T, matchQuery string, date string, topN string, opts QueryOpts) TSDBStatusResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/status/tsdb", app.httpListenAddr, opts.getTenant())
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

	res, statusCode := app.cli.PostForm(t, seriesURL, values)
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

// GraphiteMetricsIndex sends a query to a /graphite/metrics/index.json
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#metrics-api
func (app *Vmselect) GraphiteMetricsIndex(t *testing.T, opts QueryOpts) GraphiteMetricsIndexResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/graphite/metrics/index.json", app.httpListenAddr, opts.getTenant())
	res, statusCode := app.cli.Get(t, seriesURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var index GraphiteMetricsIndexResponse
	if err := json.Unmarshal([]byte(res), &index); err != nil {
		t.Fatalf("could not unmarshal metrics index response data:\n%s\n err: %v", res, err)
	}
	return index
}

// APIV1AdminTenants sends a query to a /admin/tenants endpoint
func (app *Vmselect) APIV1AdminTenants(t *testing.T) *AdminTenantsResponse {
	t.Helper()

	tenantsURL := fmt.Sprintf("http://%s/admin/tenants", app.httpListenAddr)
	res, statusCode := app.cli.Get(t, tenantsURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var tenants *AdminTenantsResponse
	if err := json.Unmarshal([]byte(res), tenants); err != nil {
		t.Fatalf("could not unmarshal tenants response data:\n%s\n err: %v", res, err)
	}

	return tenants
}

// String returns the string representation of the vmselect app state.
func (app *Vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
