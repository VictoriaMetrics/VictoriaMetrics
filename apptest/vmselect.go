package apptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// MustStartVmselect is a test helper function that starts an instance of
// vmselect and fails the test if the app fails to start.
func MustStartVmselect(t *testing.T, instance string, flags []string, cli *Client) *Vmselect {
	t.Helper()

	app, err := StartVmselect(instance, flags, cli)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
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

// PrometheusAPIV1Series sends a query to a /prometheus/api/v1/series endpoint
// and returns the list of time series that match the query.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1series
func (app *Vmselect) PrometheusAPIV1Series(t *testing.T, tenant, matchQuery string) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/select/%s/prometheus/api/v1/series", app.httpListenAddr, tenant)
	values := url.Values{}
	values.Add("match[]", matchQuery)
	jsonRes := app.cli.PostForm(t, seriesURL, values, http.StatusOK)
	var res PrometheusAPIV1SeriesResponse
	if err := json.Unmarshal([]byte(jsonRes), &res); err != nil {
		t.Fatalf("could not unmarshal /prometheus/api/v1/series response: %v", err)
	}
	return &res
}

// String returns the string representation of the vmselect app state.
func (app *Vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
