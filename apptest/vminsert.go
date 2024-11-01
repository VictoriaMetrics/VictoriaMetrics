package apptest

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Vminsert holds the state of a vminsert app and provides vminsert-specific
// functions.
type Vminsert struct {
	*app
	*ServesMetrics

	httpListenAddr string
	cli            *Client
}

// MustStartVminsert is a test helper function that starts an instance of
// vminsert and fails the test if the app fails to start.
func MustStartVminsert(t *testing.T, instance string, flags []string, cli *Client) *Vminsert {
	t.Helper()

	app, err := StartVminsert(instance, flags, cli)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

// StartVminsert starts an instance of vminsert with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVminsert(instance string, flags []string, cli *Client) (*Vminsert, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/vminsert", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Vminsert{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr: stderrExtracts[0],
		cli:            cli,
	}, nil
}

// PrometheusAPIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to
// /prometheus/api/v1/import/prometheus vminsert endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1importprometheus
func (app *Vminsert) PrometheusAPIV1ImportPrometheus(t *testing.T, tenant string, records []string) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/prometheus", app.httpListenAddr, tenant)
	app.cli.Post(t, url, "text/plain", strings.Join(records, "\n"), http.StatusNoContent)
}

// String returns the string representation of the vminsert app state.
func (app *Vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
