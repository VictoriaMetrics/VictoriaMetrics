package apptest

import (
	"io"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// StartVmauth starts the latest version of vmauth.
//
// The path to the binary can be provided via VMAUTH_PATH environment
// variable. If the variable is not set, ../../bin/vmauth-race will be
// used.
func StartVmauth(instance string, flags []string, cli *Client, configFilePath string, output io.Writer) (*Vmauth, error) {
	binary := os.Getenv("VMAUTH_PATH")
	if binary == "" {
		binary = "../../bin/vmauth-race"
	}
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
			"-auth.config":    configFilePath,
		},
		extractREs: []*regexp.Regexp{
			vmauthHttpListenAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return newVmauth(app, cli, configFilePath, vmauthRuntimeValues{
		httpListenAddr: stderrExtracts[0],
	}), nil
}

type vmauthRuntimeValues struct {
	httpListenAddr string
}

func newVmauth(app *app, cli *Client, configFilePath string, rt vmauthRuntimeValues) *Vmauth {
	return &Vmauth{
		app:            app,
		metricsClient:  newMetricsClient(cli, rt.httpListenAddr),
		httpListenAddr: rt.httpListenAddr,
		configFilePath: configFilePath,
		cli:            cli,
	}
}

// Vmauth holds the state of a vmauth app and provides vmauth-specific
// functions.
type Vmauth struct {
	*app
	*metricsClient

	cli            *Client
	httpListenAddr string
	configFilePath string
}

// GetHTTPListenAddr returns listen http addr
func (app *Vmauth) GetHTTPListenAddr() string {
	return app.httpListenAddr
}

// UpdateConfiguration updates the vmauth configuration file with the provided YAML content,
// sends SIGHUP to trigger config reload
// and waits until vmauth_config_last_reload_total increases.
// Fails the test if no reload is detected within 2 seconds.
func (app *Vmauth) UpdateConfiguration(t *testing.T, configFileYAML string) {
	t.Helper()

	fs.MustWriteSync(app.configFilePath, []byte(configFileYAML))

	prevTotal := app.GetIntMetric(t, "vmauth_config_last_reload_total")

	if err := app.process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("unexpected signal error: %s", err)
	}

	var currTotal int
	for range 20 {
		currTotal = app.GetIntMetric(t, "vmauth_config_last_reload_total")
		if currTotal > prevTotal {
			return
		}

		time.Sleep(time.Millisecond * 100)
	}

	t.Fatalf("config were not reloaded after SIGHUP signal; previous total: %d, current total: %d", prevTotal, currTotal)
}
