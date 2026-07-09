package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
)

// StartVmselect starts the latest version of vmselect.
//
// The path to the binary can be provided via VMSELECT_PATH environment
// variable. If the variable is not set, ../../bin/vmselect-race will be
// used.
func StartVmselect(instance string, flags []string, cli *Client, output io.Writer) (*Vmselect, error) {
	binary := os.Getenv("VMSELECT_PATH")
	if binary == "" {
		binary = "../../bin/vmselect-race"
	}
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-clusternativeListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
			vmselectAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return newVmselect(app, cli, vmselectRuntimeValues{
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
	}), nil
}

type vmselectRuntimeValues struct {
	httpListenAddr          string
	clusternativeListenAddr string
}

func newVmselect(app *app, cli *Client, rt vmselectRuntimeValues) *Vmselect {
	return &Vmselect{
		app:           app,
		metricsClient: newMetricsClient(cli, rt.httpListenAddr),
		vmselectClient: &vmselectClient{
			cli: cli,
			url: func(op, path string, opts QueryOpts) string {
				return getClusterPath(rt.httpListenAddr, op, path, opts)
			},
			metricNamesStatsResetURL: fmt.Sprintf("http://%s/admin/api/v1/admin/status/metric_names_stats/reset", rt.httpListenAddr),
			tenantsURL:               fmt.Sprintf("http://%s/admin/tenants", rt.httpListenAddr),
		},
		httpListenAddr:          rt.httpListenAddr,
		clusternativeListenAddr: rt.clusternativeListenAddr,
	}
}

// Vmselect holds the state of a vmselect app and provides vmselect-specific
// functions.
type Vmselect struct {
	*app
	*metricsClient
	*vmselectClient

	httpListenAddr          string
	clusternativeListenAddr string
}

// ClusternativeListenAddr returns the address at which the vmselect process is
// listening for connections from other vmselect apps.
func (app *Vmselect) ClusternativeListenAddr() string {
	return app.clusternativeListenAddr
}

// HTTPAddr returns the address at which the vmselect process is
// listening for incoming HTTP requests.
func (app *Vmselect) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vmselect app state.
func (app *Vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
