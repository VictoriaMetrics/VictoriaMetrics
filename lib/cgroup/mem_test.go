package cgroup

import (
	"math"
	"os"
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

func TestNormalizeMemoryLimitV1(t *testing.T) {
	f := func(limit, want int64) {
		t.Helper()
		if got := normalizeMemoryLimitV1(limit); got != want {
			t.Fatalf("unexpected result for %d; got %d; want %d", limit, got, want)
		}
	}
	// finite
	f(1073741824, 1073741824)
	// zero
	f(0, 0)
	// negative
	f(-1, 0)
	// unlimited
	f(math.MaxInt64&^(int64(os.Getpagesize())-1), 0)
	// max
	f(math.MaxInt64, 0)
}
