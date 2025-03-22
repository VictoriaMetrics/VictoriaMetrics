package apptest

import (
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

	forceFlushURL string
	forceMergeURL string
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

		forceFlushURL: fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
		forceMergeURL: fmt.Sprintf("http://%s/internal/force_merge", stderrExtracts[1]),
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

	_, statusCode := app.cli.Get(t, app.forceFlushURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (app *Vmstorage) ForceMerge(t *testing.T) {
	t.Helper()
	_, statusCode := app.cli.Get(t, app.forceMergeURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// String returns the string representation of the vmstorage app state.
func (app *Vmstorage) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q vminsertAddr: %q vmselectAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr, app.vminsertAddr, app.vmselectAddr}...)
}
