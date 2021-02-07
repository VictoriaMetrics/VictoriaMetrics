package cgroup

import (
	"testing"
)

func TestCountCPUs(t *testing.T) {
	f := func(s string, nExpected int) {
		t.Helper()
		n := countCPUs(s)
		if n != nExpected {
			t.Fatalf("unexpected result from countCPUs(%q); got %d; want %d", s, n, nExpected)
		}
	}
	f("", -1)
	f("1", 1)
	f("234", 1)
	f("1,2", 2)
	f("0-1", 2)
	f("0-0", 1)
	f("1-2,3,5-9,200-210", 19)
	f("0-3", 4)
	f("0-6", 7)
}

func TestGetCPUStatQuota(t *testing.T) {
	f := func(sysPath, cgroupPath string, want int64, wantErr bool) {
		t.Helper()
		got, err := getCPUStat(sysPath, cgroupPath, "cpu.cfs_quota_us")
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unxpected error value: %v, want err: %v", err, wantErr)
		}
		if got != want {
			t.Fatalf("unxpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/", "testdata/self/cgroup", -1, false)
	f("testdata/cgroup", "testdata/self/cgroup", 10, false)
	f("testdata/", "testdata/missing_folder", 0, true)
}

func TestGetCPUStatPeriod(t *testing.T) {
	f := func(sysPath, cgroupPath string, want int64, wantErr bool) {
		t.Helper()
		got, err := getCPUStat(sysPath, cgroupPath, "cpu.cfs_period_us")
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unxpected error value: %v, want err: %v", err, wantErr)
		}
		if got != want {
			t.Fatalf("unxpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/", "testdata/self/cgroup", 100000, false)
	f("testdata/cgroup", "testdata/self/cgroup", 500000, false)
	f("testdata/", "testdata/missing_folder", 0, true)
}
