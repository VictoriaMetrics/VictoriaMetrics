package apptest

import (
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
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
	*metricsClient

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

	app, stderrExtracts, err := startApp(instance, "../../bin/vmauth-race", flags, &appOptions{
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

	// The harness only confirms readiness of the builtin-routes server via the
	// pprof log line. When -httpInternalListenAddr is set, the builtin routes
	// are served on the internal address while the public -httpListenAddr is a
	// separate server started in its own goroutine. That public server may not
	// be accepting connections yet when startApp returns, which makes tests
	// that immediately query it flaky (connection refused) and can even trip a
	// shutdown panic in vmauth. Wait until the public address is reachable.
	if publicAddr := explicitListenAddr(flags, "-httpListenAddr"); publicAddr != "" && publicAddr != stderrExtracts[0] {
		if err := waitForTCPListener(publicAddr, 5*time.Second); err != nil {
			app.Stop()
			return nil, err
		}
	}

	return &Vmauth{
		app:            app,
		metricsClient:  newMetricsClient(cli, stderrExtracts[0]),
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

// explicitListenAddr returns the value of the given listen-addr flag (e.g.
// -httpListenAddr) if it is set to a concrete address. It returns an empty
// string when the flag is absent or bound to a random port (host:0), since
// such addresses cannot be dialed before the app reports them.
func explicitListenAddr(flags []string, name string) string {
	prefix := name + "="
	for _, f := range flags {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		addr := strings.TrimPrefix(f, prefix)
		if addr == "" || strings.HasSuffix(addr, ":0") {
			return ""
		}
		return addr
	}
	return ""
}

// waitForTCPListener waits until addr accepts a TCP connection or the timeout
// elapses, in which case it returns an error.
func waitForTCPListener(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for TCP listener at %q: %w", timeout, addr, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
