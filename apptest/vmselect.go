package apptest

import (
	"fmt"
	"regexp"
	"testing"
)

type vmselect struct {
	*app
	httpListenAddr string
}

func mustStartVmselect(t *testing.T, instance string, flags []string) *vmselect {
	t.Helper()

	app, err := startVmselect(instance, flags)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

func startVmselect(instance string, flags []string) (*vmselect, error) {
	app, stderrExtracts, err := startApp(instance, "../bin/vmselect", flags, &appOptions{
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

	return &vmselect{
		app:            app,
		httpListenAddr: stderrExtracts[0],
	}, nil
}

func (app *vmselect) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
