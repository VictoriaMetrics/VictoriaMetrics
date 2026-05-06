package apptest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/golang/snappy"
)

// Vmagent holds the state of a vmagent app and provides vmagent-specific functions
type Vmagent struct {
	*app
	*ServesMetrics

	httpListenAddr string
}

// StartVmagent starts an instance of vmagent with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmagent(instance string, flags []string, cli *Client, promScrapeConfigFilePath string, output io.Writer) (*Vmagent, error) {
	extractREs := []*regexp.Regexp{
		httpListenAddrRE,
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vmagent-race", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-promscrape.config":       promScrapeConfigFilePath,
			"-remoteWrite.tmpDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
		},
		extractREs: extractREs,
		output:     output,
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
		httpListenAddr: stderrExtracts[0],
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
func (app *Vmagent) APIV1ImportPrometheusNoWaitFlush(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))
	headers := opts.getHeaders()
	headers.Set("Content-Type", "text/plain")
	url := getVMAgentInsertPath(app.httpListenAddr, "prometheus/api/v1/import/prometheus", opts)
	_, statusCode := app.cli.Post(t, url, data, headers)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// getVMAgentInsertPath returns URL path for writes.
// If tenant is set in QueryOpts, it will return cluster-like path for ingestion.
// If tenant is empty, it will return single-node (no tenants) path.
func getVMAgentInsertPath(addr, suffix string, o QueryOpts) string {
	if o.Tenant != "" {
		// QueryOpts.Tenant has priority over headers
		return fmt.Sprintf("http://%s/insert/%s/%s", addr, o.Tenant, suffix)
	}

	h := o.getHeaders()
	if h.Get("AccountID") != "" || h.Get("ProjectID") != "" {
		// vmagent supports tenantID in HTTP headers only if -enableMultitenantHandlers and -enableMultitenancyViaHeaders are set
		// see https://docs.victoriametrics.com/victoriametrics/vmagent/#multitenancy
		return fmt.Sprintf("http://%s/insert/%s", addr, suffix)
	}

	// tenant is missing in QueryOpts and in HTTP headers. Use single-node (no tenants) path
	return fmt.Sprintf("http://%s/%s", addr, suffix)
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

// RemoteWriteSamplesDropped sums up the total number of dropped remote write samples for given remote write URL.
func (app *Vmagent) RemoteWriteSamplesDropped(t *testing.T, url string) int {
	re := regexp.MustCompile(fmt.Sprintf("vmagent_remotewrite_samples_dropped_total{.*url=%q.*}", url))
	total := 0.0
	for _, v := range app.GetMetricsByRegexp(t, re) {
		total += v
	}
	return int(total)
}

// RemoteWritePendingInmemoryBlocks sums up the total number of pending in-memory blocks for given remote write URL.
func (app *Vmagent) RemoteWritePendingInmemoryBlocks(t *testing.T, url string) int {
	re := regexp.MustCompile(fmt.Sprintf("vmagent_remotewrite_pending_inmemory_blocks{.*url=%q.*}", url))
	total := 0.0
	for _, v := range app.GetMetricsByRegexp(t, re) {
		total += v
	}
	return int(total)
}

// RemoteWriteRequests sums up the total number of sending requests for given remote write URL.
func (app *Vmagent) RemoteWriteRequests(t *testing.T, url string) int {
	re := regexp.MustCompile(fmt.Sprintf("vmagent_remotewrite_requests_total{.*url=%q.*}", url))
	total := 0.0
	for _, v := range app.GetMetricsByRegexp(t, re) {
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
	for range 30 {
		currTotal = app.GetMetric(t, "vmagent_relabel_config_reloads_total")
		if currTotal > prevTotal {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("relabel configs were not reloaded after SIGHUP signal; previous total: %f, current total: %f", prevTotal, currTotal)
}

// PrometheusAPIV1Write is a test helper function that inserts a
// collection of records in Prometheus remote-write format by sending a HTTP
// POST request to /prometheus/api/v1/write vmagent endpoint.
func (app *Vmagent) PrometheusAPIV1Write(t *testing.T, wr prompb.WriteRequest, opts QueryOpts) {
	t.Helper()

	url := getVMAgentInsertPath(app.httpListenAddr, "prometheus/api/v1/write", opts)
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	recordsCount := len(wr.Timeseries)
	if prommetadata.IsEnabled() {
		recordsCount += len(wr.Metadata)
	}
	headers := opts.getHeaders()
	headers.Set("Content-Type", "application/x-protobuf")
	app.sendBlocking(t, recordsCount, func() {
		_, statusCode := app.cli.Post(t, url, data, headers)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// HTTPAddr returns the address at which the vmagent process is listening
// for http connections.
func (app *Vmagent) HTTPAddr() string {
	return app.httpListenAddr
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
func (app *Vmagent) sendBlocking(t *testing.T, _ int, send func()) {
	t.Helper()

	currRowsSentCount := app.remoteWriteRequestsTotal(t)

	send()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)
	// TODO: properly account wantRowsSentCount
	// currently vmagent doesn't expose per time-series write information
	// so we can only account number of blocks sent via remote write protocol
	// it should be suitable for tests purpose
	wantRowsSentCount := currRowsSentCount + 1
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
