package apptest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"
)

// Vmsingle holds the state of a vmsingle app and provides vmsingle-specific
// functions.
type Vmsingle struct {
	*app
	*ServesMetrics
	*vmselectClient
	*vminsertClient

	storageDataPath string
	httpListenAddr  string

	// vmstorage URLs.
	forceFlushURL string
	forceMergeURL string
}

// StartVmsingleAt starts an instance of vmsingle with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr).
func StartVmsingleAt(instance, binary string, flags []string, cli *Client, output io.Writer) (*Vmsingle, error) {
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath":    fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":     "127.0.0.1:0",
			"-graphiteListenAddr": ":0",
			"-opentsdbListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			graphiteListenAddrRE,
			openTSDBListenAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return &Vmsingle{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		vmselectClient: &vmselectClient{
			vmselectCli: cli,
			url: func(op, path string, opts QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", stderrExtracts[1], path)
			},
			metricNamesStatsResetURL: fmt.Sprintf("http://%s/api/v1/admin/status/metric_names_stats/reset", stderrExtracts[1]),
			tenantsURL:               "vmsingle-does-not-serve-tenants",
		},
		vminsertClient: &vminsertClient{
			vminsertCli: cli,
			url: func(_, path string, _ QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", stderrExtracts[1], path)
			},
			openTSDBURL: func(_, path string, _ QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", stderrExtracts[3], path)
			},
			graphiteListenAddr: stderrExtracts[2],
			sendBlocking: func(t *testing.T, _ int, send func()) {
				t.Helper()
				send()
			},
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],

		forceFlushURL: fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
		forceMergeURL: fmt.Sprintf("http://%s/internal/force_merge", stderrExtracts[1]),
	}, nil
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (app *Vmsingle) ForceFlush(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceFlushURL, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (app *Vmsingle) ForceMerge(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceMergeURL, nil)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// SnapshotCreate creates a database snapshot by sending a query to the
// /snapshot/create endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotCreate(t *testing.T) *SnapshotCreateResponse {
	t.Helper()

	data, statusCode := app.cli.Post(t, app.SnapshotCreateURL(), nil, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotCreateResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotCreateURL returns the URL for creating snapshots.
func (app *Vmsingle) SnapshotCreateURL() string {
	return fmt.Sprintf("http://%s/snapshot/create", app.httpListenAddr)
}

// APIV1AdminTSDBSnapshot creates a database snapshot by sending a query to the
// /api/v1/admin/tsdb/snapshot endpoint.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot.
func (app *Vmsingle) APIV1AdminTSDBSnapshot(t *testing.T) *APIV1AdminTSDBSnapshotResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/api/v1/admin/tsdb/snapshot", app.httpListenAddr)
	data, statusCode := app.cli.Post(t, queryURL, nil, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res APIV1AdminTSDBSnapshotResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal prometheus snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotList lists existing database snapshots by sending a query to the
// /snapshot/list endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotList(t *testing.T) *SnapshotListResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/list", app.httpListenAddr)
	data, statusCode := app.cli.Get(t, queryURL, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotListResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot list response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDelete deletes a snapshot by sending a query to the
// /snapshot/delete endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotDelete(t *testing.T, snapshotName string) *SnapshotDeleteResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/delete?snapshot=%s", app.httpListenAddr, snapshotName)
	data, statusCode := app.cli.Delete(t, queryURL)
	wantStatusCodes := map[int]bool{
		http.StatusOK:                  true,
		http.StatusInternalServerError: true,
	}
	if !wantStatusCodes[statusCode] {
		t.Fatalf("unexpected status code: got %d, want %v, resp text=%q", statusCode, wantStatusCodes, data)
	}

	var res SnapshotDeleteResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDeleteAll deletes all snapshots by sending a query to the
// /snapshot/delete_all endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotDeleteAll(t *testing.T) *SnapshotDeleteAllResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/delete_all", app.httpListenAddr)
	data, statusCode := app.cli.Get(t, queryURL, nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotDeleteAllResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete all response: data=%q, err: %v", data, err)
	}

	return &res
}

// HTTPAddr returns the address at which the vminsert process is
// listening for incoming HTTP requests.
func (app *Vmsingle) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vmsingle app state.
func (app *Vmsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
