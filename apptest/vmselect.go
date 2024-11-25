package apptest

import (
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

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a HTTP POST request to
// /prometheus/api/v1/export vmselect endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1export
func (app *Vmselect) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	exportURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/export", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")
	res := app.cli.PostForm(t, exportURL, values, http.StatusOK)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a HTTP POST request to /prometheus/api/v1/query
// vmselect endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query
func (app *Vmselect) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/query", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("query", query)

	res := app.cli.PostForm(t, queryURL, values, http.StatusOK)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a HTTP POST request to
// /prometheus/api/v1/query_range vmselect endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query_range
func (app *Vmselect) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/query_range", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("query", query)

	res := app.cli.PostForm(t, queryURL, values, http.StatusOK)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Series sends a query to a /prometheus/api/v1/series endpoint
// and returns the list of time series that match the query.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1series
func (app *Vmselect) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/series", app.httpListenAddr, opts.getTenant())
	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	res := app.cli.PostForm(t, seriesURL, values, http.StatusOK)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// String returns the string representation of the vmselect app state.
func (app *Vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
