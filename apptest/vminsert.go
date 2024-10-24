package apptest

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

type vminsert struct {
	*app
	*servesMetrics
	httpListenAddr string
	cli            *client
}

func mustStartVminsert(t *testing.T, instance string, flags []string, cli *client) *vminsert {
	t.Helper()

	app, err := startVminsert(instance, flags, cli)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

func startVminsert(instance string, flags []string, cli *client) (*vminsert, error) {
	app, stderrExtracts, err := startApp(instance, "../bin/vminsert", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &vminsert{
		app: app,
		servesMetrics: &servesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr: stderrExtracts[0],
		cli:            cli,
	}, nil
}

func (app *vminsert) prometheusAPIV1ImportPrometheus(t *testing.T, tenant string, records []string) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/prometheus", app.httpListenAddr, tenant)
	app.cli.post(t, url, "text/plain", strings.Join(records, "\n"), http.StatusNoContent)
}

func (app *vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
