package cgroup

import (
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// UpdateGOMAXPROCSToCPUQuota updates GOMAXPROCS to cgroup CPU quota if GOMAXPROCS isn't set in environment var.
//
// This function must be called after logger.Init().
func UpdateGOMAXPROCSToCPUQuota() {
	if v := os.Getenv("GOMAXPROCS"); v != "" {
		// Do not override explicitly set GOMAXPROCS.
		logger.Infof("using GOMAXPROCS=%q set via environment variable", v)
		return
	}
	q := getCPUQuota()
	if q <= 0 {
		// Do not change GOMAXPROCS
		return
	}
	gomaxprocs := int(q + 0.5)
	numCPU := runtime.NumCPU()
	if gomaxprocs > numCPU {
		// There is no sense in setting more GOMAXPROCS than the number of available CPU cores.
		logger.Infof("cgroup CPU quota=%d exceeds NumCPU=%d; using GOMAXPROCS=NumCPU", gomaxprocs, numCPU)
		return
	}
	if gomaxprocs <= 0 {
		gomaxprocs = 1
	}
	logger.Infof("updating GOMAXPROCS to %d according to cgroup CPU quota", gomaxprocs)
	runtime.GOMAXPROCS(gomaxprocs)
}

func getCPUQuota() float64 {
	quotaUS, err := readInt64("/sys/fs/cgroup/cpu/cpu.cfs_quota_us", "cat /sys/fs/cgroup/cpu$(cat /proc/self/cgroup | grep cpu, | cut -d: -f3)/cpu.cfs_quota_us")
	if err != nil {
		return 0
	}
	if quotaUS <= 0 {
		// The quota isn't set. This may be the case in multilevel containers.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
		return getOnlineCPUCount()
	}
	periodUS, err := readInt64("/sys/fs/cgroup/cpu/cpu.cfs_period_us", "cat /sys/fs/cgroup/cpu$(cat /proc/self/cgroup | grep cpu, | cut -d: -f3)/cpu.cfs_period_us")
	if err != nil {
		return 0
	}
	return float64(quotaUS) / float64(periodUS)
}

func getOnlineCPUCount() float64 {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
	data, err := ioutil.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return -1
	}
	n := float64(countCPUs(string(data)))
	if n <= 0 {
		return -1
	}
	return n
}

func countCPUs(data string) int {
	data = strings.TrimSpace(data)
	n := 0
	for _, s := range strings.Split(data, ",") {
		n++
		if !strings.Contains(s, "-") {
			if _, err := strconv.Atoi(s); err != nil {
				return -1
			}
			continue
		}
		bounds := strings.Split(s, "-")
		if len(bounds) != 2 {
			return -1
		}
		start, err := strconv.Atoi(bounds[0])
		if err != nil {
			return -1
		}
		end, err := strconv.Atoi(bounds[1])
		if err != nil {
			return -1
		}
		n += end - start
	}
	return n
}
