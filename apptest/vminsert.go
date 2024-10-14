package apptest

import (
	"fmt"
	"regexp"
	"testing"
)

type vminsert struct {
	*app
	httpListenAddr string
}

func mustStartVminsert(t *testing.T, instance string, flags []string) *vminsert {
	t.Helper()

	app, err := startVminsert(instance, flags)
	if err != nil {
		t.Fatalf("Could not start %s: %v", instance, err)
	}

	return app
}

func startVminsert(instance string, flags []string) (*vminsert, error) {
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
		app:            app,
		httpListenAddr: stderrExtracts[0],
	}, nil
}

func (app *vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
