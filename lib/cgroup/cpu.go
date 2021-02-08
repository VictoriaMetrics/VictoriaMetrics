package cgroup

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// AvailableCPUs returns the number of available CPU cores for the app.
func AvailableCPUs() int {
	availableCPUsOnce.Do(updateGOMAXPROCSToCPUQuota)
	return runtime.GOMAXPROCS(-1)
}

var availableCPUsOnce sync.Once

// updateGOMAXPROCSToCPUQuota updates GOMAXPROCS to cgroup CPU quota if GOMAXPROCS isn't set in environment var.
func updateGOMAXPROCSToCPUQuota() {
	if v := os.Getenv("GOMAXPROCS"); v != "" {
		// Do not override explicitly set GOMAXPROCS.
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
		return
	}
	if gomaxprocs <= 0 {
		gomaxprocs = 1
	}
	runtime.GOMAXPROCS(gomaxprocs)
}

func getCPUStat(sysPath, cgroupPath, statName string) (int64, error) {
	n, err := readInt64(path.Join(sysPath, statName))
	if err == nil {
		return n, nil
	}
	subPath, err := grepFirstMatch(cgroupPath, "cpu,", 2, ":")
	if err != nil {
		return 0, err
	}
	return readInt64(path.Join(sysPath, subPath, statName))
}

func getCPUQuota() float64 {
	quotaUS, err := getCPUStat("/sys/fs/cgroup/cpu", "/proc/self/cgroup", "cpu.cfs_quota_us")
	if err != nil {
		return 0
	}
	if quotaUS <= 0 {
		// The quota isn't set. This may be the case in multilevel containers.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
		return getOnlineCPUCount()
	}
	periodUS, err := getCPUStat("/sys/fs/cgroup/cpu", "/proc/self/cgroup", "cpu.cfs_period_us")
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
