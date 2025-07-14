package memory

import (
	"bufio"
	"bytes"
	"errors"
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
	am, err := getAvailableMemory()
	if err != nil {
		return 0
	}
	return am
	//mem := cgroup.GetMemoryUsage()
	//if mem <= 0 || int64(int(mem)) != mem || int(mem) > usedMem {
	//	mem = cgroup.GetHierarchicalMemoryUsage()
	//	if mem <= 0 || int64(int(mem)) != mem || int(mem) > usedMem {
	//		return usedMem
	//	}
	//}
	//return int(mem)
}

// getAvailableMemory parse /proc/meminfo and return MemAvailable in byte.
func getAvailableMemory() (int, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	s := bufio.NewScanner(bytes.NewReader(b))
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if fields[0] != "MemAvailable:" {
			continue
		}
		val, err := strconv.ParseInt(fields[1], 0, 64)
		if err != nil {
			return 0, err
		}
		switch len(fields) {
		case 2:
			return int(val), nil
		case 3:
			if fields[2] != "kB" {
				return 0, fmt.Errorf("%w: unsupported unit in optional 3rd field %q", ErrFileParse, fields[2])
			}
			return int(1024 * val), nil
		default:
			return 0, fmt.Errorf("%w: malformed line %q", ErrFileParse, s.Text())
		}
	}
	return 0, fmt.Errorf("AvailableMemory not found")
}

var (
	ErrFileParse = errors.New("error parsing file")
)
