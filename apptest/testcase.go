package apptest

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// TestCase holds the state and defines clean-up procedure common for all test
// cases.
type TestCase struct {
	t   *testing.T
	cli *Client

	startedApps []Stopper
}

// Stopper is an interface of objects that needs to be stopped via Stop() call
type Stopper interface {
	Stop()
}

// NewTestCase creates a new test case.
func NewTestCase(t *testing.T) *TestCase {
	return &TestCase{t, NewClient(), nil}
}

// Dir returns the directory name that should be used by as the -storageDataDir.
func (tc *TestCase) Dir() string {
	return tc.t.Name()
}

// Client returns an instance of the client that can be used for interacting with
// the app(s) under test.
func (tc *TestCase) Client() *Client {
	return tc.cli
}

// Stop performs the test case clean up, such as closing all client connections
// and removing the -storageDataDir directory.
//
// Note that the -storageDataDir is not removed in case of test case failure to
// allow for further manual debugging.
func (tc *TestCase) Stop() {
	tc.cli.CloseConnections()
	for _, app := range tc.startedApps {
		app.Stop()
	}
	if !tc.t.Failed() {
		fs.MustRemoveAll(tc.Dir())
	}
}

// MustStartVmsingle is a test helper function that starts an instance of
// vmsingle and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmsingle(instance string, flags []string) *Vmsingle {
	tc.t.Helper()

	app, err := StartVmsingle(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(app)
	return app
}

// MustStartVmstorage is a test helper function that starts an instance of
// vmstorage and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmstorage(instance string, flags []string) *Vmstorage {
	tc.t.Helper()

	app, err := StartVmstorage(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(app)
	return app
}

// MustStartVmselect is a test helper function that starts an instance of
// vmselect and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmselect(instance string, flags []string) *Vmselect {
	tc.t.Helper()

	app, err := StartVmselect(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(app)
	return app
}

// MustStartVminsert is a test helper function that starts an instance of
// vminsert and fails the test if the app fails to start.
func (tc *TestCase) MustStartVminsert(instance string, flags []string) *Vminsert {
	tc.t.Helper()

	app, err := StartVminsert(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(app)
	return app
}

func (tc *TestCase) addApp(app Stopper) {
	tc.startedApps = append(tc.startedApps, app)
}
