package cgroup

import (
	"os"
	"runtime/debug"
	"strconv"
)

// GetGOGC returns GOGC value for the currently running process.
//
// See https://golang.org/pkg/runtime/#hdr-Environment_Variables for more details about GOGC
func GetGOGC() int {
	return gogc
}

// SetGOGC sets GOGC to the given value unless it is already set via environment variable.
func SetGOGC(gogcNew int) {
	if v := os.Getenv("GOGC"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			n = 100
		}
		gogc = int(n)
	} else {
		gogc = gogcNew
		debug.SetGCPercent(gogcNew)
	}
}

var gogc int

// GetMemoryLimit returns cgroup memory limit
func GetMemoryLimit() int64 {
	// Try determining the amount of memory inside docker container.
	// See https://stackoverflow.com/questions/42187085/check-mem-limit-within-a-docker-container
	//
	// Read memory limit according to https://unix.stackexchange.com/questions/242718/how-to-find-out-how-much-memory-lxc-container-is-allowed-to-consume
	// This should properly determine the limit inside lxc container.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/84
	n, err := getMemStat("memory.limit_in_bytes")
	if err == nil {
		return n
	}
	n, err = getMemStatV2("memory.max")
	if err != nil {
		return 0
	}
	return n
}

func getMemStatV2(statName string) (int64, error) {
	// See https: //www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#memory-interface-files
	return getStatGeneric(statName, "/sys/fs/cgroup", "/proc/self/cgroup", "")
}

func getMemStat(statName string) (int64, error) {
	return getStatGeneric(statName, "/sys/fs/cgroup/memory", "/proc/self/cgroup", "memory")
}

// GetHierarchicalMemoryLimit returns hierarchical memory limit
// https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
func GetHierarchicalMemoryLimit() int64 {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/699
	n, err := getHierarchicalMemoryLimit("/sys/fs/cgroup/memory", "/proc/self/cgroup")
	if err != nil {
		return 0
	}
	return n
}

func getHierarchicalMemoryLimit(sysfsPrefix, cgroupPath string) (int64, error) {
	data, err := getFileContents("memory.stat", sysfsPrefix, cgroupPath, "memory")
	if err != nil {
		return 0, err
	}
	memStat, err := grepFirstMatch(data, "hierarchical_memory_limit", 1, " ")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(memStat, 10, 64)
}
