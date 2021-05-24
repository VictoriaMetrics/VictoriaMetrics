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

func TestGetCPUQuotaV2(t *testing.T) {
	f := func(sysPrefix, cgroupPath string, expectedCPU float64) {
		t.Helper()
		got, err := getCPUQuotaV2(sysPrefix, cgroupPath)
		if err != nil {
			t.Fatalf("unexpected error: %s, sysPrefix: %s, cgroupPath: %s", err, sysPrefix, cgroupPath)
		}
		if got != expectedCPU {
			t.Fatalf("unexpected result from getCPUQuotaV2(%s, %s), got %f, want %f", sysPrefix, cgroupPath, got, expectedCPU)
		}
	}
	f("testdata/cgroup", "testdata/self/cgroupv2", 2)
}
