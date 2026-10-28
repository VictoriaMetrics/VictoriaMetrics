package cgroup

import (
	"os"
	"path"
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

func TestV1ServiceLimitWithReadableRoot(t *testing.T) {
	for _, tc := range []struct {
		name, root, service string
	}{
		{"memory.limit_in_bytes", "9223372036854771712\n", "60442841088\n"},
		{"memory.stat", "hierarchical_memory_limit 9223372036854771712\n", "hierarchical_memory_limit 60442841088\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			service := path.Join(root, "system.slice", "vmstorage.service")
			if err := os.MkdirAll(service, 0755); err != nil {
				t.Fatal(err)
			}
			proc := path.Join(root, "proc-self-cgroup")
			for name, content := range map[string]string{
				path.Join(root, tc.name):    tc.root,
				path.Join(service, tc.name): tc.service,
				proc:                        "5:memory:/system.slice/vmstorage.service\n",
			} {
				if err := os.WriteFile(name, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := getFileContents(tc.name, root, proc, "memory")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.service {
				t.Fatalf("got %q; want process cgroup value %q", got, tc.service)
			}
		})
	}
}
