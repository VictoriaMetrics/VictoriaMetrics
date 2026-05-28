package apptest

import (
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"
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
