package cgroup

import "testing"

func TestGetMemLimit(t *testing.T) {
	f := func(sysPath, cgroupPath string, want int64, wantErr bool) {
		t.Helper()
		got, err := getMemLimit(sysPath, cgroupPath)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unxpected error: %v, wantErr: %v", err, wantErr)
		}
		if got != want {
			t.Fatalf("unxpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/", "testdata/self/cgroup", 9223372036854771712, false)
	f("testdata/cgroup", "testdata/self/cgroup", 523372036854771712, false)
	f("testdata/", "testdata/none_existing_folder", 0, true)
}

func TestGetMemHierarchical(t *testing.T) {
	f := func(sysPath, cgroupPath string, want int64, wantErr bool) {
		t.Helper()
		got, err := getHierarchicalMemoryLimit(sysPath, cgroupPath)
		if (err != nil && !wantErr) || (err == nil && wantErr) {
			t.Fatalf("unxpected error: %v, wantErr: %v", err, wantErr)
		}
		if got != want {
			t.Fatalf("unxpected result, got: %d, want %d", got, want)
		}
	}
	f("testdata/", "testdata/self/cgroup", 16, false)
	f("testdata/cgroup", "testdata/self/cgroup", 120, false)
	f("testdata/", "testdata/none_existing_folder", 0, true)
}
