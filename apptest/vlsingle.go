package apptest

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// Vlsingle holds the state of a vlsingle app and provides vlsingle-specific
// functions.
type Vlsingle struct {
	*app
	*ServesMetrics

	storageDataPath string
	httpListenAddr  string

	forceFlushURL string
}

// StartVlsingle starts an instance of vlsingle with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr).
func StartVlsingle(instance string, flags []string, cli *Client) (*Vlsingle, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/victoria-logs", flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":  "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			logsStorageDataPathRE,
			httpListenAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Vlsingle{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],

		forceFlushURL: fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
	}, nil
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (app *Vlsingle) ForceFlush(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceFlushURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// JSONLineWrite is a test helper function that inserts a
// collection of records in json line format by sending a HTTP
// POST request to /insert/jsonline vlsingle endpoint.
//
// See https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api
func (app *Vlsingle) JSONLineWrite(t *testing.T, records []string, opts QueryOptsLogs) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))

	url := fmt.Sprintf("http://%s/insert/jsonline", app.httpListenAddr)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}

	_, statusCode := app.cli.Post(t, url, "text/plain", data)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// NativeWrite is a test helper function that sends a collection of records
// to /internal/insert API.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vlinsert/internalinsert/internalinsert.go
func (app *Vlsingle) NativeWrite(t *testing.T, records []logstorage.InsertRow, opts QueryOpts) {
	t.Helper()
	var data []byte
	for _, record := range records {
		data = record.Marshal(data)
	}
	dstURL := fmt.Sprintf("http://%s/internal/insert", app.httpListenAddr)
	uv := opts.asURLValues()
	uv.Add("version", "v1")
	dstURL += "?" + uv.Encode()

	app.cli.Post(t, dstURL, "application/octet-stream", data)
}

// LogsQLQuery is a test helper function that performs
// PromQL/MetricsQL range query by sending a HTTP POST request to
// /select/logsql/query endpoint.
//
// See https://docs.victoriametrics.com/victorialogs/querying/#querying-logs
func (app *Vlsingle) LogsQLQuery(t *testing.T, query string, opts QueryOptsLogs) *LogsQLQueryResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("query", query)

	url := fmt.Sprintf("http://%s/select/logsql/query", app.httpListenAddr)
	res, _ := app.cli.PostForm(t, url, values)
	return NewLogsQLQueryResponse(t, res)
}

// HTTPAddr returns the address at which the vmstorage process is listening
// for http connections.
func (app *Vlsingle) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vlsingle app state.
func (app *Vlsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
