package cgroup

import (
	"fmt"
	"io/ioutil"
	"os"
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

func getCPUQuota() float64 {
	cpuQuota, err := getCPUStatGeneric()
	if err != nil {
		return 0
	}

	if cpuQuota <= 0 {
		// The quota isn't set. This may be the case in multilevel containers.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
		return getOnlineCPUCount()
	}
	return cpuQuota
}

func getCPUStatGeneric() (float64, error) {
	quotaUS, err := getCPUStat("cpu.cfs_quota_us")
	if err == nil {
		periodUS, err := getCPUStat("cpu.cfs_period_us")
		if err == nil {
			return float64(quotaUS) / float64(periodUS), nil
		}
	}
	return getCPUStatV2("/sys/fs/cgroup", "/proc/self/cgroup")
}

func getCPUStat(statName string) (int64, error) {
	return getStatGeneric(statName, "/sys/fs/cgroup/cpu", "/proc/self/cgroup", "cpu,")
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

func getCPUStatV2(sysPrefix, cgroupPath string) (float64, error) {
	data, err := getFileContents("cpu.max", sysPrefix, cgroupPath, "")
	if err != nil {
		return 0, err
	}
	return parseCPUMax(data)
}

// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#cpu
func parseCPUMax(data string) (float64, error) {
	data = strings.TrimRight(data, "\r\n")
	bounds := strings.Split(data, " ")
	if len(bounds) != 2 {
		return 0, fmt.Errorf("unexpected count: %d, want quota and period, got: %s", len(bounds), data)
	}
	if bounds[0] == "max" {
		return -1, nil
	}
	quota, err := strconv.ParseUint(bounds[0], 10, 64)
	if err != nil {
		return 0, err
	}
	period, err := strconv.ParseUint(bounds[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return float64(quota) / float64(period), nil
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
