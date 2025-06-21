package apptest

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Vlagent holds the state of a vlagent app and provides vlagent-specific functions
type Vlagent struct {
	*app
	*ServesMetrics

	remoteStoragesCount int
	httpListenAddr      string
}

// StartVlagent starts an instance of vlagent with the given flags.
// It also sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVlagent(instance string, remoteWriteURLs []string, flags []string, cli *Client) (*Vlagent, error) {
	extractREs := []*regexp.Regexp{
		httpListenAddrRE,
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vlagent", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":            "127.0.0.1:0",
			"-remoteWrite.url":           strings.Join(remoteWriteURLs, ","),
			"-remoteWrite.tmpDataPath":   fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-remoteWrite.flushInterval": "10ms",
			"-remoteWrite.showURL":       "true",
		},
		extractREs: extractREs,
	})
	if err != nil {
		return nil, err
	}

	return &Vlagent{
		app:                 app,
		remoteStoragesCount: len(remoteWriteURLs),
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr: stderrExtracts[0],
	}, nil
}

// JSONLineWrite is a test helper function that inserts a
// collection of records in json line format by sending a HTTP
// POST request to /insert/jsonline vlagent endpoint.
//
// See https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api
func (app *Vlagent) JSONLineWrite(t *testing.T, records []string, opts QueryOptsLogs) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))

	url := fmt.Sprintf("http://%s/insert/jsonline", app.httpListenAddr)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "text/plain", data)
		if statusCode != http.StatusOK {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
		}
	})
}

// WaitQueueEmptyAfter checks that persistent queue is empty
// after execution of provided callback
func (app *Vlagent) WaitQueueEmptyAfter(t *testing.T, cb func()) {
	t.Helper()
	const (
		retries = 70
		period  = 100 * time.Millisecond
	)
	// vlagent_remotewrite_blocks_sent_total
	// take in account data replication
	blocksSent := app.remoteWriteBlocksSent(t)
	cb()
	for range retries {
		if app.remoteWriteBlocksSent(t) > blocksSent && app.persistentQueueSize(t) == 0 {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted logs to be flushed to remote storage")

}

// sendBlocking sends the data to remote write url by executing `send` function and
// waits until the data is actually sent.
//
// vlagent does not send the data immediately. It first puts the data into a
// buffer. Then a background goroutine takes the data from the buffer sends it
// to the vmstorage. This happens every 1s by default.
//
// Waiting is implemented a retrieving the value of `vlagent_remotewrite_block_size_rows_sum`
// metric and checking whether it is equal or greater than the wanted value.
// If it is, then the data has been sent to remote storage.
//
// Unreliable if the records are inserted concurrently.
func (app *Vlagent) sendBlocking(t *testing.T, numRecordsToSend int, send func()) {
	t.Helper()

	send()

	const (
		retries = 50
		period  = 100 * time.Millisecond
	)
	// take in account data replication
	wantRowsSentCount := app.remoteWriteRowsPushed(t) + numRecordsToSend*app.remoteStoragesCount
	for range retries {
		if app.remoteWriteRowsPushed(t) >= wantRowsSentCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted rows to be sent to remote storage")
}

func (app *Vlagent) remoteWriteBlocksSent(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vlagent_remotewrite_blocks_sent_total") {
		total += v
	}
	return int(total)
}

func (app *Vlagent) remoteWriteRowsPushed(t *testing.T) int {
	total := 0.0
	// vlagent_remotewrite_blocks_sent_total
	for _, v := range app.GetMetricsByPrefix(t, "vlagent_remotewrite_block_size_rows_sum") {
		total += v
	}
	return int(total)
}

func (app *Vlagent) persistentQueueSize(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vlagent_remotewrite_pending_data_bytes") {
		total += v
	}
	for _, v := range app.GetMetricsByPrefix(t, "vlagent_remotewrite_pending_inmemory_blocks") {
		total += v
	}
	return int(total)
}
