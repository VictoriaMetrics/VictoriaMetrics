package apptest

import (
	"fmt"
	"io"
	"regexp"
)

// Vmselect holds the state of a vmselect app and provides vmselect-specific
// functions.
type Vmselect struct {
	*app
	*metricsClient
	*vmselectClient

	httpListenAddr          string
	clusternativeListenAddr string
	cli                     *Client
}

// StartVmselect starts an instance of vmselect with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmselectAt(instance, binary string, flags []string, cli *Client, output io.Writer) (*Vmselect, error) {
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

	return &Vmselect{
		app:           app,
		metricsClient: newMetricsClient(cli, stderrExtracts[0]),
		vmselectClient: &vmselectClient{
			vmselectCli: cli,
			url: func(op, path string, opts QueryOpts) string {
				return getClusterPath(stderrExtracts[0], op, path, opts)
			},
			metricNamesStatsResetURL: fmt.Sprintf("http://%s/admin/api/v1/admin/status/metric_names_stats/reset", stderrExtracts[0]),
			tenantsURL:               fmt.Sprintf("http://%s/admin/tenants", stderrExtracts[0]),
		},
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
		cli:                     cli,
	}, nil
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
