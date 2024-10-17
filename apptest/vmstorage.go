package apptest

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"
)

type vmstorage struct {
	*app
	*servesMetrics
	storageDataPath string
	httpListenAddr  string
	vminsertAddr    string
	vmselectAddr    string
}

func mustStartVmstorage(t *testing.T, instance string, flags []string, cli *client) *vmstorage {
	t.Helper()

	app, err := startVmstorage(instance, flags, cli)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

func startVmstorage(instance string, flags []string, cli *client) (*vmstorage, error) {
	app, stderrExtracts, err := startApp(instance, "../bin/vmstorage", flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":  "127.0.0.1:0",
			"-vminsertAddr":    "127.0.0.1:0",
			"-vmselectAddr":    "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			vminsertAddrRE,
			vmselectAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &vmstorage{
		app: app,
		servesMetrics: &servesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],
		vminsertAddr:    stderrExtracts[2],
		vmselectAddr:    stderrExtracts[3],
	}, nil
}

func (app *vmstorage) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q vminsertAddr: %q vmselectAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr, app.vminsertAddr, app.vmselectAddr}...)
}
