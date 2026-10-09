package cgroup

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// AvailableCPUs returns the number of available CPU cores for the app.
//
// The number is rounded to the next integer value if fractional number of CPU cores are available.
func AvailableCPUs() int {
	return runtime.GOMAXPROCS(-1)
}

func init() {
	cpuCoresHost := getOnlineCPUCount()

	var effectiveCPUQuota float64
	cpuCoresQuota := getCPUQuotaGeneric()
	if cpuCoresQuota < 0 {
		// Fall back to online CPUs when the CPU quota cannot be read or is unset.
		// This may be the case in multilevel containers.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
		effectiveCPUQuota = cpuCoresHost
	} else {
		effectiveCPUQuota = cpuCoresQuota
	}

	if effectiveCPUQuota > 0 {
		updateGOMAXPROCSToCPUQuota(effectiveCPUQuota)
	}

	cpuCoresAvailable := float64(runtime.NumCPU())
	if effectiveCPUQuota > 0 && cpuCoresAvailable > effectiveCPUQuota {
		cpuCoresAvailable = effectiveCPUQuota
	}

	if cpuCoresHost <= 0 {
		cpuCoresHost = float64(runtime.NumCPU())
	}
	metrics.NewGauge(`process_cpu_cores_host`, func() float64 {
		return cpuCoresHost
	})
	metrics.NewGauge(`process_cpu_cores_cgroup_quota`, func() float64 {
		return cpuCoresQuota
	})
	metrics.NewGauge(`process_cpu_cores_available`, func() float64 {
		return cpuCoresAvailable
	})
}

// updateGOMAXPROCSToCPUQuota updates GOMAXPROCS to cpuQuota if GOMAXPROCS isn't set in environment var.
func updateGOMAXPROCSToCPUQuota(cpuQuota float64) {
	if v := os.Getenv("GOMAXPROCS"); v != "" {
		// Do not override explicitly set GOMAXPROCS.
		return
	}

	// Round gomaxprocs to the floor of cpuQuota, since Go runtime doesn't work well
	// with fractional available CPU cores.
	gomaxprocs := int(cpuQuota)
	if gomaxprocs <= 0 {
		gomaxprocs = 1
	}
	if cpuQuota > float64(gomaxprocs) {
		logger.Warnf("rounding CPU quota %.1f to %d CPUs for performance reasons - see https://docs.victoriametrics.com/victoriametrics/bestpractices/#kubernetes", cpuQuota, gomaxprocs)
	}

	numCPU := runtime.NumCPU()
	if gomaxprocs > numCPU {
		// There is no sense in setting more GOMAXPROCS than the number of available CPU cores.
		gomaxprocs = numCPU
	}

	runtime.GOMAXPROCS(gomaxprocs)
}

// getCPUQuotaGeneric returns the cgroup CPU quota in CPU cores.
// It returns -1 if the quota cannot be read, is invalid, or is unset.
func getCPUQuotaGeneric() float64 {
	quota, err := getCPUQuotaV1()
	if err == nil {
		return quota
	}

	quota, err = getCPUQuotaV2("/sys/fs/cgroup", "/proc/self/cgroup")
	if err != nil || quota <= 0 {
		return -1
	}
	return quota
}

func getCPUStat(statName string) (int64, error) {
	return getStatGeneric(statName, "/sys/fs/cgroup/cpu", "/proc/self/cgroup", "cpu,")
}

func getOnlineCPUCount() float64 {
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/685#issuecomment-674423728
	data, err := os.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return -1
	}
	n := float64(countCPUs(string(data)))
	if n <= 0 {
		return -1
	}
	return n
}

func getCPUQuotaV1() (float64, error) {
	quotaUS, err := getCPUStat("cpu.cfs_quota_us")
	if err != nil {
		return -1, err
	}
	periodUS, err := getCPUStat("cpu.cfs_period_us")
	if err != nil {
		return -1, err
	}
	if quotaUS <= 0 || periodUS <= 0 {
		return -1, nil
	}
	return float64(quotaUS) / float64(periodUS), nil
}

// See https://www.freedesktop.org/software/systemd/man/latest/systemd.slice.html
func getCPUQuotaV2(sysfsPrefix, cgroupPath string) (float64, error) {
	subPath, err := readCgroupV2SubPath(cgroupPath)
	if err != nil {
		subPath = "/"
	}
	var minQuota float64 = -1
	for {
		// travers sub path hierarchy and use a minimal value for stat
		data, err := os.ReadFile(path.Join(sysfsPrefix, subPath, "cpu.max"))
		if err == nil {
			quota, err := parseCPUMax(strings.TrimSpace(string(data)))
			if err != nil {
				return 0, fmt.Errorf("cannot parse cpu.max at %s: %w", subPath, err)
			}
			if quota > 0 && (minQuota < 0 || quota < minQuota) {
				minQuota = quota
			}
		}
		if subPath == "/" || subPath == "." {
			break
		}
		subPath = path.Dir(subPath)
	}
	return minQuota, nil
}

// See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#cpu
func parseCPUMax(data string) (float64, error) {
	bounds := strings.Split(data, " ")
	if len(bounds) > 2 {
		return 0, fmt.Errorf("unexpected line format: want 'quota period'; got: %s", data)
	}
	if bounds[0] == "max" {
		return -1, nil
	}
	quota, err := strconv.ParseUint(bounds[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse quota: %w", err)
	}
	// The default is “max 100000”.
	period := uint64(100_000)
	if len(bounds) == 2 {
		period, err = strconv.ParseUint(bounds[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse period: %w", err)
		}
		if period == 0 {
			return 0, fmt.Errorf("zero value for period is not allowed")
		}
	}
	return float64(quota) / float64(period), nil
}

func countCPUs(data string) int {
	data = strings.TrimSpace(data)
	n := 0
	for s := range strings.SplitSeq(data, ",") {
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
