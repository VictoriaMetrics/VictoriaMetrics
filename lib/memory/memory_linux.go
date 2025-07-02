package memory

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	mem := cgroup.GetMemoryLimit()
	if mem <= 0 || int64(int(mem)) != mem || int(mem) > totalMem {
		// Try reading hierarchical memory limit.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/699
		mem = cgroup.GetHierarchicalMemoryLimit()
		if mem <= 0 || int64(int(mem)) != mem || int(mem) > totalMem {
			return totalMem
		}
	}
	return int(mem)
}

func sysCurrentMemory() int {
	ms, err := getMemStats("/proc/self/status")
	if err != nil {
		return 0
	}

	return int(ms.rssAnon)
	//mem := cgroup.GetMemoryUsage()
	//if mem <= 0 || int64(int(mem)) != mem || int(mem) > usedMem {
	//	mem = cgroup.GetHierarchicalMemoryUsage()
	//	if mem <= 0 || int64(int(mem)) != mem || int(mem) > usedMem {
	//		return usedMem
	//	}
	//}
	//return int(mem)
}

func getMemStats(path string) (*memStats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ms memStats
	lines := strings.Split(string(data), "\n")
	for _, s := range lines {
		if !strings.HasPrefix(s, "Vm") && !strings.HasPrefix(s, "Rss") {
			continue
		}
		// Extract key value.
		line := strings.Fields(s)
		if len(line) != 3 {
			return nil, fmt.Errorf("unexpected number of fields found in %q; got %d; want %d", s, len(line), 3)
		}
		memStatName := line[0]
		memStatValue := line[1]
		value, err := strconv.ParseUint(memStatValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse number from %q: %w", s, err)
		}
		if line[2] != "kB" {
			return nil, fmt.Errorf("expecting kB value in %q; got %q", s, line[2])
		}
		value *= 1024
		switch memStatName {
		case "VmPeak:":
			ms.vmPeak = value
		case "VmHWM:":
			ms.rssPeak = value
		case "RssAnon:":
			ms.rssAnon = value
		case "RssFile:":
			ms.rssFile = value
		case "RssShmem:":
			ms.rssShmem = value
		}
	}
	return &ms, nil
}

// https://man7.org/linux/man-pages/man5/procfs.5.html
type memStats struct {
	vmPeak   uint64
	rssPeak  uint64
	rssAnon  uint64
	rssFile  uint64
	rssShmem uint64
}
