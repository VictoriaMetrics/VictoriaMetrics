package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"
	"time"
)

// Vmsingle holds the state of a vmsingle app and provides vmsingle-specific
// functions.
type Vmsingle struct {
	*app
	*metricsClient
	*vmstorageClient
	*vmselectClient
	*vminsertClient

	storageDataPath string
	httpListenAddr  string
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
		app:           app,
		metricsClient: newMetricsClient(cli, stderrExtracts[1]),
		vmstorageClient: &vmstorageClient{
			vmstorageCli:   cli,
			httpListenAddr: stderrExtracts[1],
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
	}, nil
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
