package memory

import (
	"math"
	"os"
	"syscall"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const maxInt = int(^uint(0) >> 1)

func sysTotalMemory() int {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		logger.Panicf("FATAL: error in syscall.Sysinfo: %s", err)
	}
	totalMem := maxInt
	if uint64(maxInt)/uint64(si.Totalram) > uint64(si.Unit) {
		totalMem = int(uint64(si.Totalram) * uint64(si.Unit))
	}
	memoryHostBytes = float64(totalMem)
	mem := cgroup.GetMemoryLimit()
	if mem <= 0 || int64(int(mem)) != mem || int(mem) > totalMem {
		// Try reading hierarchical memory limit.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/699
		if hmem := cgroup.GetHierarchicalMemoryLimit(); isCgroupMemoryLimitSet(hmem) {
			mem = hmem
		}
	}
	if isCgroupMemoryLimitSet(mem) {
		memoryCgroupBytes = float64(mem)
	}
	if mem <= 0 || int64(int(mem)) != mem || int(mem) > totalMem {
		return totalMem
	}
	return int(mem)
}

// isCgroupMemoryLimitSet returns whether mem is a real cgroup memory limit.
// Cgroup v1 reports math.MaxInt64 rounded down to the page size when no limit is set.
func isCgroupMemoryLimitSet(mem int64) bool {
	pageSize := int64(os.Getpagesize())
	noLimit := math.MaxInt64 / pageSize * pageSize
	return mem > 0 && mem < noLimit
}
