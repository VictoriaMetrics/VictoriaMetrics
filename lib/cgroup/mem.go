package cgroup

import (
	"fmt"
	"os"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
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
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func getMemStatV2(statName string) (int64, error) {
	// See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#memory-interface-files
	return getMemLimitV2("/sys/fs/cgroup", "/proc/self/cgroup", statName)
}

func getMemLimitV2(sysfsPrefix, cgroupPath, statName string) (int64, error) {
	subPath, err := readCgroupV2SubPath(cgroupPath)
	if err != nil {
		subPath = "/"
	}
	var minLimit int64 = -1
	for {
		// travers sub path hierarchy and use a minimal value for stat
		data, err := os.ReadFile(path.Join(sysfsPrefix, subPath, statName))
		if err == nil {
			s := strings.TrimSpace(string(data))
			if s != "max" {
				n, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("cannot parse %s at %s: %w", statName, subPath, err)
				}
				if n > 0 && (minLimit < 0 || n < minLimit) {
					minLimit = n
				}
			}
		}
		if subPath == "/" || subPath == "." {
			break
		}
		subPath = path.Dir(subPath)
	}
	return minLimit, nil
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
