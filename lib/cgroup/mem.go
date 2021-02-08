package cgroup

import (
	"strconv"
)

// GetMemoryLimit returns cgroup memory limit
func GetMemoryLimit() int64 {
	// Try determining the amount of memory inside docker container.
	// See https://stackoverflow.com/questions/42187085/check-mem-limit-within-a-docker-container
	//
	// Read memory limit according to https://unix.stackexchange.com/questions/242718/how-to-find-out-how-much-memory-lxc-container-is-allowed-to-consume
	// This should properly determine the limit inside lxc container.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/84
	n, err := getMemStat("memory.limit_in_bytes")
	if err != nil {
		return 0
	}
	return n
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
