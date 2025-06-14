package apptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"
)

// Vmstorage holds the state of a vmstorage app and provides vmstorage-specific
// functions.
type Vmstorage struct {
	*app
	*ServesMetrics

	storageDataPath string
	httpListenAddr  string
	vminsertAddr    string
	vmselectAddr    string
}

// StartVmstorage starts an instance of vmstorage with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmstorage(instance string, flags []string, cli *Client) (*Vmstorage, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/vmstorage", flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":  "127.0.0.1:0",
			"-vminsertAddr":    "127.0.0.1:0",
			"-vmselectAddr":    "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			vminsertAddrRE,
			vmselectAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Vmstorage{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],
		vminsertAddr:    stderrExtracts[2],
		vmselectAddr:    stderrExtracts[3],
	}, nil
}

// VminsertAddr returns the address at which the vmstorage process is listening
// for vminsert connections.
func (app *Vmstorage) VminsertAddr() string {
	return app.vminsertAddr
}

// VmselectAddr returns the address at which the vmstorage process is listening
// for vmselect connections.
func (app *Vmstorage) VmselectAddr() string {
	return app.vmselectAddr
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (app *Vmstorage) ForceFlush(t *testing.T) {
	t.Helper()

	forceFlushURL := fmt.Sprintf("http://%s/internal/force_flush", app.httpListenAddr)
	_, statusCode := app.cli.Get(t, forceFlushURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (app *Vmstorage) ForceMerge(t *testing.T) {
	t.Helper()

	forceMergeURL := fmt.Sprintf("http://%s/internal/force_merge", app.httpListenAddr)
	_, statusCode := app.cli.Get(t, forceMergeURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// SnapshotCreate creates a database snapshot by sending a query to the
// /snapshot/create endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmstorage) SnapshotCreate(t *testing.T) *SnapshotCreateResponse {
	t.Helper()

	data, statusCode := app.cli.Post(t, app.SnapshotCreateURL(), "", nil)
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
func (app *Vmstorage) SnapshotCreateURL() string {
	return fmt.Sprintf("http://%s/snapshot/create", app.httpListenAddr)
}

// SnapshotList lists existing database snapshots by sending a query to the
// /snapshot/list endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmstorage) SnapshotList(t *testing.T) *SnapshotListResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/list", app.httpListenAddr)
	data, statusCode := app.cli.Get(t, queryURL)
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
func (app *Vmstorage) SnapshotDelete(t *testing.T, snapshotName string) *SnapshotDeleteResponse {
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
func (app *Vmstorage) SnapshotDeleteAll(t *testing.T) *SnapshotDeleteAllResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/delete_all", app.httpListenAddr)
	data, statusCode := app.cli.Post(t, queryURL, "", nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotDeleteAllResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete all response: data=%q, err: %v", data, err)
	}

	return &res
}

// String returns the string representation of the vmstorage app state.
func (app *Vmstorage) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q vminsertAddr: %q vmselectAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr, app.vminsertAddr, app.vmselectAddr}...)
}
