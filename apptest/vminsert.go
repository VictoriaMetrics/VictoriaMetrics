package apptest

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Vminsert holds the state of a vminsert app and provides vminsert-specific
// functions.
type Vminsert struct {
	*app
	*ServesMetrics

	httpListenAddr string
	cli            *Client
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
func (app *Vminsert) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/prometheus", app.httpListenAddr, opts.Tenant)
	wantRowsSentCount := app.rpcRowsSentTotal(t) + len(records)
	app.cli.Post(t, url, "text/plain", strings.Join(records, "\n"), http.StatusNoContent)
	app.waitUntilSent(t, wantRowsSentCount)
}

// String returns the string representation of the vminsert app state.
func (app *Vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}

// waitUntilSent waits until vminsert sends buffered data to vmstorage.
//
// Waiting is implemented a retrieving the value of `vm_rpc_rows_sent_total`
// metric and checking whether it is equal or greater than the wanted value.
// If it is, then the data has been sent to vmstorage.
//
// Unreliable if the records are inserted concurrently.
func (app *Vminsert) waitUntilSent(t *testing.T, wantRowsSentCount int) {
	t.Helper()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)

	for range retries {
		if app.rpcRowsSentTotal(t) >= wantRowsSentCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted rows to be sent to vmstorage")
}

// rpcRowsSentTotal retrieves the values of all vminsert
// `vm_rpc_rows_sent_total` metrics (there will be one for each vmstorage) and
// returns their integer sum.
func (app *Vminsert) rpcRowsSentTotal(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vm_rpc_rows_sent_total") {
		total += v
	}
	return int(total)
}
