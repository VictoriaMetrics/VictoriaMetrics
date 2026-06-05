package apptest

// MustStartVmsingle_v1_132_0 is a test helper function that starts an instance
// of vmsingle-v1.132.0 (last version that uses legacy index) and fails the test
// if the app fails to start.
func (tc *TestCase) MustStartVmsingle_v1_132_0(instance string, flags []string) *Vmsingle {
	tc.t.Helper()

	app, err := StartVmsingle_v1_132_0(instance, flags, tc.cli, tc.output)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}

// MustStartVmstorage_v1_132_0 is a test helper function that starts an instance
// of vmstorage-v1.132.0 (last version that uses legacy index)  and fails the
// test if the app fails to start.
func (tc *TestCase) MustStartVmstorage_v1_132_0(instance string, flags []string) *Vmstorage {
	tc.t.Helper()

	app, err := StartVmstorage_v1_132_0(instance, flags, tc.cli, tc.output)
	if err != nil {
		tc.t.Fatalf("Could not start %s: %v", instance, err)
	}
	tc.addApp(instance, app)
	return app
}

// MustStartCluster_v1_132_0 starts a cluster with vmstorage-v1.132.0 with
// custom flags.
func (tc *TestCase) MustStartCluster_v1_132_0(opts *ClusterOptions) *Vmcluster {
	tc.t.Helper()

	vmstorage1 := tc.MustStartVmstorage_v1_132_0(opts.Vmstorage1Instance, opts.Vmstorage1Flags)
	vmstorage2 := tc.MustStartVmstorage_v1_132_0(opts.Vmstorage2Instance, opts.Vmstorage2Flags)

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
