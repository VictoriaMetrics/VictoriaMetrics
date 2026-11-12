package cgroup

import (
	"fmt"
	"math"
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
	n, err := getMemLimit("/sys/fs/cgroup/memory", "/proc/self/cgroup", "memory", "memory.limit_in_bytes")
	if err == nil && n > 0 {
		return n
	}
	// Try cgroupv2, it contains a single line at /proc/self/cgroup, so there is no need to grep anything from it.
	// 0::/vm.slice/vmagent.service - for example.
	// See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#memory-interface-files
	n, err = getMemLimit("/sys/fs/cgroup", "/proc/self/cgroup", "", "memory.max")
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func getMemLimit(sysfsPrefix, cgroupPath, cgroupGrepLine, statName string) (int64, error) {
	subPath := readCgroupSubPath(cgroupPath, cgroupGrepLine)
	var minLimit int64 = -1
	for _, dir := range cgroupDirs(sysfsPrefix, subPath) {
		data, err := os.ReadFile(path.Join(dir, statName))
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(data))
		if s == "max" {
			continue
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %s at %s: %w", statName, dir, err)
		}
		n = normalizeMemoryLimitV1(n)
		if n > 0 && (minLimit < 0 || n < minLimit) {
			minLimit = n
		}
	}
	return minLimit, nil
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
	subPath := readCgroupSubPath(cgroupPath, "memory")
	found := false
	var minLimit int64
	for _, dir := range cgroupDirs(sysfsPrefix, subPath) {
		data, err := os.ReadFile(path.Join(dir, "memory.stat"))
		if err != nil {
			continue
		}
		found = true
		memStat, err := grepFirstMatch(string(data), "hierarchical_memory_limit", 1, " ")
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(memStat, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse hierarchical_memory_limit at %s: %w", dir, err)
		}
		n = normalizeMemoryLimitV1(n)
		if n > 0 && (minLimit == 0 || n < minLimit) {
			minLimit = n
		}
	}
	if !found {
		return 0, fmt.Errorf("cannot find memory.stat for %q at %q", subPath, sysfsPrefix)
	}
	return minLimit, nil
}

func normalizeMemoryLimitV1(n int64) int64 {
	// Cgroup v1 uses math.MaxInt64 rounded down to the page size for an unlimited value.
	pageSize := int64(os.Getpagesize())
	noLimit := math.MaxInt64 / pageSize * pageSize
	if n <= 0 || n >= noLimit {
		return 0
	}
	return n
}
