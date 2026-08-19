package cgroup

import (
	"testing"
)

func TestIsCPUCgroupLimited(t *testing.T) {
	f := func(cpuQuota float64, cpuCount int, expected bool) {
		t.Helper()
		got := isCPUCgroupLimited(cpuQuota, cpuCount)
		if got != expected {
			t.Fatalf("unexpected result from isCPUCgroupLimited(%f, %d); got %v; want %v", cpuQuota, cpuCount, got, expected)
		}
	}

	f(-1, 8, false)
	f(0, 8, false)
	f(8, 8, false)
	f(16, 8, false)
	f(7.5, 8, true)
	f(0.5, 8, true)
}

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
	f("testdata/cgroup/cpu_unset", "", -1)
	f("testdata/cgroup/cpu_onlymax", "", 2)

	// systemd slice
	f("testdata/v2slice", "testdata/self/cgroupv2_slice", 2)
}
