package apptest

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	t.Parallel()
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

// MustStartVmagent is a test helper function that starts an instance of
// vmagent and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmagent(instance string, flags []string, promScrapeConfigFileYAML string) *Vmagent {
	tc.t.Helper()

	promScrapeConfigFilePath := path.Join(tc.t.TempDir(), "prometheus.yml")
	if err := os.WriteFile(promScrapeConfigFilePath, []byte(promScrapeConfigFileYAML), os.ModePerm); err != nil {
		tc.t.Fatalf("cannot init vmagent: prom config file write failed: %s", err)
	}
	app, err := StartVmagent(instance, flags, tc.cli, promScrapeConfigFilePath)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}

// Vmcluster represents a typical cluster setup: several vmstorage replicas, one
// vminsert, and one vmselect.
//
// Both Vmsingle and Vmcluster implement the PrometheusWriteQuerier used in
// business logic tests to abstract out the infrasture.
//
// This type is not suitable for infrastructure tests where custom cluster
// setups are often required.
type Vmcluster struct {
	*Vminsert
	*Vmselect
	Vmstorages []*Vmstorage
}

// ForceFlush forces the ingested data to become visible for searching
// immediately.
func (c *Vmcluster) ForceFlush(t *testing.T) {
	for _, s := range c.Vmstorages {
		s.ForceFlush(t)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (c *Vmcluster) ForceMerge(t *testing.T) {
	for _, s := range c.Vmstorages {
		s.ForceMerge(t)
	}
}

// MustStartVmauth is a test helper function that starts an instance of
// vmauth and fails the test if the app fails to start.
func (tc *TestCase) MustStartVmauth(instance string, flags []string, configFileYAML string) *Vmauth {
	tc.t.Helper()

	configFilePath := path.Join(tc.t.TempDir(), "config.yaml")
	if err := os.WriteFile(configFilePath, []byte(configFileYAML), os.ModePerm); err != nil {
		tc.t.Fatalf("cannot init vmauth: config file write failed: %s", err)
	}
	app, err := StartVmauth(instance, flags, tc.cli, configFilePath)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}

// MustStartVmbackup is a test helper that starts an instance of vmbackup
// and waits until the app exits. It fails the test if the app fails to start or
// exits with non zero code.
func (tc *TestCase) MustStartVmbackup(instance, storageDataPath, snapshotCreateURL, dst string) {
	tc.t.Helper()

	if err := StartVmbackup(instance, storageDataPath, snapshotCreateURL, dst); err != nil {
		tc.t.Fatalf("vmbackup %q failed to start or exited with non-zero code: %v", instance, err)
	}

	// Do not add the process to the list of running apps using
	// tc.addApp(instance, app), because the method blocks until the process
	// exits.
}

// MustStartVmrestore is a test helper that starts an instance of vmrestore
// and waits until the app exits. It fails the test if the app fails to start or
// exits with non zero code.
func (tc *TestCase) MustStartVmrestore(instance, src, storageDataPath string) {
	tc.t.Helper()

	if err := StartVmrestore(instance, src, storageDataPath); err != nil {
		tc.t.Fatalf("vmrestore %q failed to start or exited with non-zero code: %v", instance, err)
	}

	// Do not add the process to the list of running apps using
	// tc.addApp(instance, app), because the method blocks until the process
	// exits.
}

// MustStartDefaultCluster starts a typical cluster configuration with default
// flags.
func (tc *TestCase) MustStartDefaultCluster() *Vmcluster {
	tc.t.Helper()

	return tc.MustStartCluster(&ClusterOptions{
		Vmstorage1Instance: "vmstorage1",
		Vmstorage2Instance: "vmstorage2",
		VminsertInstance:   "vminsert",
		VmselectInstance:   "vmselect",
	})
}

// ClusterOptions holds the params for simple cluster configuration suitable for
// most tests.
//
// The cluster consists of two vmstorages, one vminsert and one vmselect, no
// data replication.
//
// Such configuration is suitable for tests that don't verify the
// cluster-specific behavior (such as sharding, replication, or multilevel
// vmselect) but instead just need a typical cluster configuration to verify
// some business logic (such as API surface, or MetricsQL). Such cluster
// tests usually come paired with corresponding vmsingle tests.
type ClusterOptions struct {
	Vmstorage1Instance string
	Vmstorage1Flags    []string
	Vmstorage2Instance string
	Vmstorage2Flags    []string
	VminsertInstance   string
	VminsertFlags      []string
	VmselectInstance   string
	VmselectFlags      []string
}

// MustStartCluster starts a typical cluster configuration with custom flags.
func (tc *TestCase) MustStartCluster(opts *ClusterOptions) *Vmcluster {
	tc.t.Helper()

	opts.Vmstorage1Flags = append(opts.Vmstorage1Flags, []string{
		"-storageDataPath=" + tc.Dir() + "/" + opts.Vmstorage1Instance,
		"-retentionPeriod=100y",
	}...)
	vmstorage1 := tc.MustStartVmstorage(opts.Vmstorage1Instance, opts.Vmstorage1Flags)

	opts.Vmstorage2Flags = append(opts.Vmstorage2Flags, []string{
		"-storageDataPath=" + tc.Dir() + "/" + opts.Vmstorage2Instance,
		"-retentionPeriod=100y",
	}...)
	vmstorage2 := tc.MustStartVmstorage(opts.Vmstorage2Instance, opts.Vmstorage2Flags)

	opts.VminsertFlags = append(opts.VminsertFlags, []string{
		"-storageNode=" + vmstorage1.VminsertAddr() + "," + vmstorage2.VminsertAddr(),
	}...)
	vminsert := tc.MustStartVminsert(opts.VminsertInstance, opts.VminsertFlags)

	opts.VmselectFlags = append(opts.VmselectFlags, []string{
		"-storageNode=" + vmstorage1.VmselectAddr() + "," + vmstorage2.VmselectAddr(),
	}...)
	vmselect := tc.MustStartVmselect(opts.VmselectInstance, opts.VmselectFlags)

	return &Vmcluster{vminsert, vmselect, []*Vmstorage{vmstorage1, vmstorage2}}
}

// MustStartVmctl is a test helper function that starts an instance of vmctl
func (tc *TestCase) MustStartVmctl(instance string, flags []string) {
	tc.t.Helper()

	err := StartVmctl(instance, flags)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
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

// StopPrometheusWriteQuerier stop all apps that are a part of the pwq.
func (tc *TestCase) StopPrometheusWriteQuerier(pwq PrometheusWriteQuerier) {
	tc.t.Helper()
	switch t := pwq.(type) {
	case *Vmsingle:
		tc.StopApp(t.Name())
	case *Vmcluster:
		tc.StopApp(t.Vminsert.Name())
		tc.StopApp(t.Vmselect.Name())
		for _, vmstorage := range t.Vmstorages {
			tc.StopApp(vmstorage.Name())
		}
	default:
		tc.t.Fatalf("Unsupported type: %v", t)
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

// MustStartDefaultVlsingle is a test helper function that starts an instance of
// vlsingle with defaults suitable for most tests.
func (tc *TestCase) MustStartDefaultVlsingle() *Vlsingle {
	tc.t.Helper()

	return tc.MustStartVlsingle("vlsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vlsingle",
		"-retentionPeriod=100y",
	})
}

// MustStartVlsingle is a test helper function that starts an instance of
// vlsingle and fails the test if the app fails to start.
func (tc *TestCase) MustStartVlsingle(instance string, flags []string) *Vlsingle {
	tc.t.Helper()

	app, err := StartVlsingle(instance, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}

// MustStartDefaultVlagent is a test helper function that starts an instance of
// vlagent with defaults suitable for most tests.
func (tc *TestCase) MustStartDefaultVlagent(remoteWriteURLs []string) *Vlagent {
	tc.t.Helper()

	return tc.MustStartVlagent("vlagent", remoteWriteURLs, nil)
}

// MustStartVlagent is a test helper function that starts an instance of
// vlagent and fails the test if the app fails to start.
func (tc *TestCase) MustStartVlagent(instance string, remoteWriteURLs []string, flags []string) *Vlagent {
	tc.t.Helper()

	app, err := StartVlagent(instance, remoteWriteURLs, flags, tc.cli)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}
