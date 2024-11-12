package apptest

import (
	"testing"
	"time"

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

type vmcluster struct {
	*Vminsert
	*Vmselect
	vmstorages []*Vmstorage
}

func (c *vmcluster) ForceFlush(t *testing.T) {
	time.Sleep(2 * time.Second)
	for _, s := range c.vmstorages {
		s.ForceFlush(t)
	}
}

// MustStartCluster is a typical cluster configuration.
//
// The cluster consists of two vmstorages, one vminsert and one vmselect, no
// data replication.
//
// Such configuration is suitable for tests that don't verify the
// cluster-specific behavior (such as sharding, replication, or multilevel
// vmselect) but instead just need a typical cluster configuration to verify
// some business logic (such as API surface, or MetricsQL). Such cluster
// tests usually come paired with corresponding vmsingle tests.
func (tc *TestCase) MustStartCluster() PrometheusWriteQuerier {
	tc.t.Helper()

	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
		"-retentionPeriod=100y",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr(),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	})

	return &vmcluster{vminsert, vmselect, []*Vmstorage{vmstorage1, vmstorage2}}
}

func (tc *TestCase) addApp(app Stopper) {
	tc.startedApps = append(tc.startedApps, app)
}
