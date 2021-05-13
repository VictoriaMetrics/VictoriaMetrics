package cgroup

import (
	"testing"
)

func TestGetStatGenericSuccess(t *testing.T) {
	f := func(statName, sysfsPrefix, cgroupPath, cgroupGrepLine string, want int64) {
		t.Helper()
		got, err := getStatGeneric(statName, sysfsPrefix, cgroupPath, cgroupGrepLine)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got != want {
			t.Fatalf("unexpected result, got: %d, want %d", got, want)
		}
	}
	f("cpu.cfs_quota_us", "testdata/", "testdata/self/cgroup", "cpu,", -1)
	f("cpu.cfs_quota_us", "testdata/cgroup", "testdata/self/cgroup", "cpu,", 10)
	f("cpu.cfs_period_us", "testdata/", "testdata/self/cgroup", "cpu,", 100000)
	f("cpu.cfs_period_us", "testdata/cgroup", "testdata/self/cgroup", "cpu,", 500000)
	f("memory.limit_in_bytes", "testdata/", "testdata/self/cgroup", "memory", 9223372036854771712)
	f("memory.limit_in_bytes", "testdata/cgroup", "testdata/self/cgroup", "memory", 523372036854771712)
	f("memory.max", "testdata/cgroup", "testdata/self/cgroupv2", "", 523372036854771712)
}

func TestGetStatGenericFailure(t *testing.T) {
	f := func(statName, sysfsPrefix, cgroupPath, cgroupGrepLine string) {
		t.Helper()
		got, err := getStatGeneric(statName, sysfsPrefix, cgroupPath, cgroupGrepLine)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if got != 0 {
			t.Fatalf("unexpected result, got: %d, want 0", got)
		}
	}
	f("cpu.cfs_quota_us", "testdata/", "testdata/missing_folder", "cpu,")
	f("cpu.cfs_period_us", "testdata/", "testdata/missing_folder", "cpu,")
	f("memory.limit_in_bytes", "testdata/", "testdata/none_existing_folder", "memory")
	f("memory.max", "testdata/", "testdata/none_existing_folder", "")
}
