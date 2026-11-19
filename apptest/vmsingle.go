package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"testing"
	"time"
)

// StartVmsingle starts the latest version of vmsingle.
//
// The path to the binary can be provided via VMSINGLE_PATH environment
// variable. If the variable is not set, ../../bin/victoria-metrics-race will be
// used.
func StartVmsingle(instance string, flags []string, cli *Client, output io.Writer) (*Vmsingle, error) {
	binary := os.Getenv("VMSINGLE_PATH")
	if binary == "" {
		binary = "../../bin/victoria-metrics-race"
	}
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath":    fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":     "127.0.0.1:0",
			"-graphiteListenAddr": "127.0.0.1:0",
			"-opentsdbListenAddr": "127.0.0.1:0",
			"-vmselectAddr":       "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			graphiteListenAddrRE,
			openTSDBListenAddrRE,
			vmselectAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return newVmsingle(app, cli, vmsingleRuntimeValues{
		storageDataPath:    stderrExtracts[0],
		httpListenAddr:     stderrExtracts[1],
		graphiteListenAddr: stderrExtracts[2],
		openTSDBListenAddr: stderrExtracts[3],
		vmselectAddr:       stderrExtracts[4],
	}), nil
}

type vmsingleRuntimeValues struct {
	storageDataPath    string
	httpListenAddr     string
	graphiteListenAddr string
	openTSDBListenAddr string
	vmselectAddr       string
}

func newVmsingle(app *app, cli *Client, rt vmsingleRuntimeValues) *Vmsingle {
	return &Vmsingle{
		app:           app,
		metricsClient: newMetricsClient(cli, rt.httpListenAddr),
		vmstorageClient: &vmstorageClient{
			cli:            cli,
			httpListenAddr: rt.httpListenAddr,
		},
		vmselectClient: &vmselectClient{
			cli: cli,
			url: func(op, path string, opts QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", rt.httpListenAddr, path)
			},
			metricNamesStatsResetURL: fmt.Sprintf("http://%s/api/v1/admin/status/metric_names_stats/reset", rt.httpListenAddr),
			tenantsURL:               "vmsingle-does-not-serve-tenants",
		},
		vminsertClient: &vminsertClient{
			cli: cli,
			url: func(_, path string, _ QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", rt.httpListenAddr, path)
			},
			openTSDBURL: func(_, path string, _ QueryOpts) string {
				return fmt.Sprintf("http://%s/%s", rt.openTSDBListenAddr, path)
			},
			graphiteListenAddr: rt.graphiteListenAddr,
			sendBlocking: func(t *testing.T, _ int, send func()) {
				t.Helper()
				send()
			},
		},
		storageDataPath: rt.storageDataPath,
		httpListenAddr:  rt.httpListenAddr,
		vmselectAddr:    rt.vmselectAddr,
	}
}

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
	vmselectAddr    string
}

// HTTPAddr returns the address at which the vminsert process is
// listening for incoming HTTP requests.
func (app *Vmsingle) HTTPAddr() string {
	return app.httpListenAddr
}

// VmselectAddr returns the address at which the vmsingle process is listening
// for vmselect connections.
func (app *Vmsingle) VmselectAddr() string {
	return app.vmselectAddr
}

// String returns the string representation of the vmsingle app state.
func (app *Vmsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
