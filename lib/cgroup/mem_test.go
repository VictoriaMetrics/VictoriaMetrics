package cgroup

import (
	"testing"
)

func TestGetHierarchicalMemoryLimitSuccess(t *testing.T) {
	f := func(sysPath, cgroupPath string, want int64) {
		t.Helper()
		got, err := getHierarchicalMemoryLimit(sysPath, cgroupPath)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got != want {
			t.Fatalf("unexpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/", "testdata/self/cgroup", 16)
	f("testdata/cgroup", "testdata/self/cgroup", 120)
}

func TestGetMemLimitV2(t *testing.T) {
	f := func(sysPrefix, cgroupPath string, want int64) {
		t.Helper()
		got, err := getMemLimitV2(sysPrefix, cgroupPath, "memory.max")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got != want {
			t.Fatalf("unexpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/cgroup", "testdata/self/cgroupv2", 523372036854771712)
	// systemd slice
	f("testdata/v2slice", "testdata/self/cgroupv2_slice", 1073741824)
}

func TestGetHierarchicalMemoryLimitFailure(t *testing.T) {
	f := func(sysPath, cgroupPath string) {
		t.Helper()
		got, err := getHierarchicalMemoryLimit(sysPath, cgroupPath)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if got != 0 {
			t.Fatalf("unexpected result, got: %d, want 0", got)
		}
	}
	f("testdata/", "testdata/none_existing_folder")
}
