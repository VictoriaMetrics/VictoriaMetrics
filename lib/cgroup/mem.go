package cgroup

import (
	"path"
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
	n, err := getMemLimit("/sys/fs/cgroup/", "/proc/self/cgroup")
	if err != nil {
		return 0
	}
	return n
}

func getMemLimit(sysPath, cgroupPath string) (int64, error) {
	n, err := readInt64(path.Join(sysPath, "memory.limit_in_bytes"))
	if err == nil {
		return n, nil
	}
	subPath, err := grepFirstMatch(cgroupPath, "memory", 2, ":")
	if err != nil {
		return 0, err
	}
	return readInt64(path.Join(sysPath, subPath, "memory.limit_in_bytes"))
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

func getHierarchicalMemoryLimit(sysPath, cgroupPath string) (int64, error) {
	n, err := getMemStatDirect(sysPath)
	if err == nil {
		return n, nil
	}
	return getMemStatSubPath(sysPath, cgroupPath)
}

func getMemStatDirect(sysPath string) (int64, error) {
	memStat, err := grepFirstMatch(path.Join(sysPath, "memory.stat"), "hierarchical_memory_limit", 1, " ")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(memStat, 10, 64)
}

func getMemStatSubPath(sysPath, cgroupPath string) (int64, error) {
	cgrps, err := grepFirstMatch(cgroupPath, "memory", 2, ":")
	if err != nil {
		return 0, err
	}
	memStat, err := grepFirstMatch(path.Join(sysPath, cgrps, "memory.stat"), "hierarchical_memory_limit", 1, " ")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(memStat, 10, 64)
}
