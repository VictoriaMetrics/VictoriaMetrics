package apptest

import (
	"fmt"
	"os"
	"regexp"
	"syscall"
	"testing"
	"time"
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
func StartVmauth(instance string, flags []string, cli *Client, configFilePath string) (*Vmauth, error) {
	extractREs := []*regexp.Regexp{
		httpBuilitinListenAddrRE,
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vmauth", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
			"-auth.config":    configFilePath,
		},
		extractREs: extractREs,
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

// UpdateConfiguration performs configuration file reload for app and waits for configuration apply
//
// Due to second prescision of config reload metric, config cannot be reloaded more than 1 time in a second
func (app *Vmauth) UpdateConfiguration(t *testing.T, configFileYAML string) {
	t.Helper()
	ct := int(time.Now().Unix())
	if err := os.WriteFile(app.configFilePath, []byte(configFileYAML), os.ModePerm); err != nil {
		t.Fatalf("unexpected error at UpdateConfiguration, cannot write configFile content: %s", err)
	}
	if err := app.process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("unexpected signal error: %s", err)
	}
	for range 10 {
		ts := app.GetIntMetric(t, "vmauth_config_last_reload_success_timestamp_seconds")
		if ts < ct {
			time.Sleep(time.Millisecond * 100)
			continue
		}
		return
	}
	t.Fatalf("timeout waiting for config reload success")
}

// GetHTTPListenAddr returns listen http addr
func (app *Vmauth) GetHTTPListenAddr() string {
	return app.httpListenAddr
}
