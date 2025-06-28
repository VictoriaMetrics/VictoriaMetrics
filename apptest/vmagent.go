package apptest

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Vmagent holds the state of a vmagent app and provides vmagent-specific functions
type Vmagent struct {
	*app
	*ServesMetrics

	httpListenAddr           string
	apiV1ImportPrometheusURL string
}

// StartVmagent starts an instance of vmagent with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmagent(instance string, flags []string, cli *Client, promScrapeConfigFilePath string) (*Vmagent, error) {
	extractREs := []*regexp.Regexp{
		httpListenAddrRE,
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vmagent", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-promscrape.config":       promScrapeConfigFilePath,
			"-remoteWrite.tmpDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
		},
		extractREs: extractREs,
	})
	if err != nil {
		return nil, err
	}

	return &Vmagent{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr:           stderrExtracts[0],
		apiV1ImportPrometheusURL: fmt.Sprintf("http://%s/api/v1/import/prometheus", stderrExtracts[0]),
	}, nil
}

// APIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to /api/v1/import/prometheus vmagent endpoint.
//
// The call is blocked until the data is flushed to vmstorage or the timeout is reached.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1importprometheus
func (app *Vmagent) APIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	app.sendBlocking(t, len(records), func() {
		app.APIV1ImportPrometheusNoWaitFlush(t, records, opts)
	})
}

// APIV1ImportPrometheusNoWaitFlush is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to /api/v1/import/prometheus vmagent endpoint.
//
// The call accepts the records but does not guarantee successful flush to vmstorage.
// Flushing may still be in progress on the function return.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1importprometheus
func (app *Vmagent) APIV1ImportPrometheusNoWaitFlush(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))
	_, statusCode := app.cli.Post(t, app.apiV1ImportPrometheusURL, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// RemoteWriteRequestsRetriesCountTotal sums up the total retries for remote write requests.
func (app *Vmagent) RemoteWriteRequestsRetriesCountTotal(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vmagent_remotewrite_retries_count_total") {
		total += v
	}
	return int(total)
}

// RemoteWritePacketsDroppedTotal sums up the total number of dropped remote write packets.
func (app *Vmagent) RemoteWritePacketsDroppedTotal(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vmagent_remotewrite_packets_dropped_total") {
		total += v
	}
	return int(total)
}

// ReloadRelabelConfigs sends SIGHUP to trigger relabel config reload
// and waits until vmagent_relabel_config_reloads_total increases.
// Fails the test if no reload is detected within 3 seconds.
func (app *Vmagent) ReloadRelabelConfigs(t *testing.T) {
	prevTotal := app.GetMetric(t, "vmagent_relabel_config_reloads_total")

	if err := app.process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("could not send SIGHUP signal to %s process: %v", app.instance, err)
	}

	var currTotal float64
	for i := 0; i < 30; i++ {
		currTotal = app.GetMetric(t, "vmagent_relabel_config_reloads_total")
		if currTotal > prevTotal {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	if currTotal <= prevTotal {
		t.Fatalf("relabel configs were not reloaded after SIGHUP signal; previous total: %f, current total: %f", prevTotal, currTotal)
	}
}

// sendBlocking sends the data to vmstorage by executing `send` function and
// waits until the data is actually sent.
//
// vmagent does not send the data immediately. It first puts the data into a
// buffer. Then a background goroutine takes the data from the buffer sends it
// to the vmstorage. This happens every 1s by default.
//
// Waiting is implemented a retrieving the value of `vmagent_remotewrite_requests_total`
// metric and checking whether it is equal or greater than the wanted value.
// If it is, then the data has been sent to vmstorage.
//
// Unreliable if the records are inserted concurrently.
func (app *Vmagent) sendBlocking(t *testing.T, numRecordsToSend int, send func()) {
	t.Helper()

	send()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)
	wantRowsSentCount := app.remoteWriteRequestsTotal(t) + numRecordsToSend
	for range retries {
		if app.remoteWriteRequestsTotal(t) >= wantRowsSentCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted rows to be sent to vmstorage")
}

func (app *Vmagent) remoteWriteRequestsTotal(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vmagent_remotewrite_requests_total") {
		total += v
	}
	return int(total)
}
