package apptest

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

// TestCase holds the state and defines clean-up procedure common for all test
// cases.
type TestCase struct {
	t   *testing.T
	cli *Client

	startedApps map[string]Stopper
}

// Stopper is an interface of objects that needs to be stopped via Stop() call
type Stopper interface {
	Stop()
}

// NewTestCase creates a new test case.
func NewTestCase(t *testing.T) *TestCase {
	return &TestCase{t, NewClient(), make(map[string]Stopper)}
}

// T returns the test state.
func (tc *TestCase) T() *testing.T {
	return tc.t
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

// MustStartDefaultVmsingle is a test helper function that starts an instance of
// vmsingle with defaults suitable for most tests.
func (tc *TestCase) MustStartDefaultVmsingle() *Vmsingle {
	tc.t.Helper()

	return tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
	})
}

// MustStartVmsingle is a test helper function that starts an instance of
// vmsingle and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmsingle(instance string, flags []string) *Vmsingle {
	tc.t.Helper()

	app, err := StartVmsingle(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
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
	tc.addApp(instance, app)
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
	tc.addApp(instance, app)
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
	tc.addApp(instance, app)
	return app
}

type vmcluster struct {
	*Vminsert
	*Vmselect
	vmstorages []*Vmstorage
}

func (c *vmcluster) ForceFlush(t *testing.T) {
	for _, s := range c.vmstorages {
		s.ForceFlush(t)
	}
}

// MustStartDefaultCluster is a typical cluster configuration suitable for most
// tests.
//
// The cluster consists of two vmstorages, one vminsert and one vmselect, no
// data replication.
//
// Such configuration is suitable for tests that don't verify the
// cluster-specific behavior (such as sharding, replication, or multilevel
// vmselect) but instead just need a typical cluster configuration to verify
// some business logic (such as API surface, or MetricsQL). Such cluster
// tests usually come paired with corresponding vmsingle tests.
func (tc *TestCase) MustStartDefaultCluster() PrometheusWriteQuerier {
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

func (tc *TestCase) addApp(instance string, app Stopper) {
	if _, alreadyStarted := tc.startedApps[instance]; alreadyStarted {
		tc.t.Fatalf("%s has already been started", instance)
	}
	tc.startedApps[instance] = app
}

// StopApp stops the app identified by the `instance` name and removes it from
// the collection of started apps.
func (tc *TestCase) StopApp(instance string) {
	if app, exists := tc.startedApps[instance]; exists {
		app.Stop()
		delete(tc.startedApps, instance)
	}
}

// ForceFlush flushes zero or more storages.
func (tc *TestCase) ForceFlush(apps ...*Vmstorage) {
	tc.t.Helper()

	for _, app := range apps {
		app.ForceFlush(tc.t)
	}
}

// AssertOptions hold the assertion params, such as got and wanted values as
// well as the message that should be included into the assertion error message
// in case of failure.
//
// In VictoriaMetrics (especially the cluster version) the inserted data does
// not become visible for querying right away. Therefore, the first comparisons
// may fail. AssertOptions allow to configure how many times the actual result
// must be retrieved and compared with the expected one and for long to wait
// between the retries. If these two params (`Retries` and `Period`) are not
// set, the default values will be used.
//
// If it is known that the data is available, then the retry functionality can
// be disabled by setting the `DoNotRetry` field.
//
// AssertOptions are used by the TestCase.Assert() method, and this method uses
// cmp.Diff() from go-cmp package for comparing got and wanted values.
// AssertOptions, therefore, allows to pass cmp.Options to cmp.Diff() via
// `CmpOpts` field.
//
// Finally the `FailNow` field controls whether the assertion should fail using
// `testing.T.Errorf()` or `testing.T.Fatalf()`.
type AssertOptions struct {
	Msg        string
	Got        func() any
	Want       any
	CmpOpts    []cmp.Option
	DoNotRetry bool
	Retries    int
	Period     time.Duration
	FailNow    bool
}

// Assert compares the actual result with the expected one possibly multiple
// times in order to account for the fact that the inserted data does not become
// available for querying right away (especially in cluster version of
// VictoriaMetrics).
func (tc *TestCase) Assert(opts *AssertOptions) {
	tc.t.Helper()

	const (
		defaultRetries = 20
		defaultPeriod  = 100 * time.Millisecond
	)

	if opts.DoNotRetry {
		opts.Retries = 1
		opts.Period = 0
	} else {
		if opts.Retries <= 0 {
			opts.Retries = defaultRetries
		}
		if opts.Period <= 0 {
			opts.Period = defaultPeriod
		}
	}

	var diff string

	for range opts.Retries {
		diff = cmp.Diff(opts.Want, opts.Got(), opts.CmpOpts...)
		if diff == "" {
			return
		}
		time.Sleep(opts.Period)
	}

	msg := fmt.Sprintf("%s (-want, +got):\n%s", opts.Msg, diff)

	if opts.FailNow {
		tc.t.Fatal(msg)
	} else {
		tc.t.Error(msg)
	}
}
