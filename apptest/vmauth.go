package apptest

import (
	"fmt"
	"io"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

var httpBuilitinListenAddrRE = regexp.MustCompile(`pprof handlers are exposed at http://(.*:\d{1,5})/debug/pprof/`)

// Vmauth holds the state of a vmauth app and provides vmauth-specific
// functions.
type Vmauth struct {
	*app
	*ServesMetrics

	httpListenAddr string
	configFilePath string
	cli            *Client
}

// StartVmauth starts an instance of vmauth with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmauth(instance string, flags []string, cli *Client, configFilePath string, output io.Writer) (*Vmauth, error) {
	extractREs := []*regexp.Regexp{
		httpBuilitinListenAddrRE,
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vmauth", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
			"-auth.config":    configFilePath,
		},
		extractREs: extractREs,
		output:     output,
	})
	if err != nil {
		return nil, err
	}

	return &Vmauth{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr: stderrExtracts[0],
		configFilePath: configFilePath,
		cli:            cli,
	}, nil
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

// GetHTTPListenAddr returns listen http addr
func (app *Vmauth) GetHTTPListenAddr() string {
	return app.httpListenAddr
}
