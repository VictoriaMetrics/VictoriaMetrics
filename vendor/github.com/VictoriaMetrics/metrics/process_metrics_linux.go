package metrics

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// See https://github.com/prometheus/procfs/blob/a4ac0826abceb44c40fc71daed2b301db498b93e/proc_stat.go#L40 .
const userHZ = 100

// Different environments may have different page size.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6457
var pageSizeBytes = uint64(os.Getpagesize())

// See http://man7.org/linux/man-pages/man5/proc.5.html
type procStat struct {
	State       byte
	Ppid        int
	Pgrp        int
	Session     int
	TtyNr       int
	Tpgid       int
	Flags       uint
	Minflt      uint
	Cminflt     uint
	Majflt      uint
	Cmajflt     uint
	Utime       uint
	Stime       uint
	Cutime      int
	Cstime      int
	Priority    int
	Nice        int
	NumThreads  int
	ItrealValue int
	Starttime   uint64
	Vsize       uint
	Rss         int
}

func writeProcessMetrics(w io.Writer) {
	statFilepath := "/proc/self/stat"
	data, err := os.ReadFile(statFilepath)
	if err != nil {
		log.Printf("ERROR: metrics: cannot open %s: %s", statFilepath, err)
		return
	}

	// Search for the end of command.
	n := bytes.LastIndex(data, []byte(") "))
	if n < 0 {
		log.Printf("ERROR: metrics: cannot find command in parentheses in %q read from %s", data, statFilepath)
		return
	}
	data = data[n+2:]

	var p procStat
	bb := bytes.NewBuffer(data)
	_, err = fmt.Fscanf(bb, "%c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&p.State, &p.Ppid, &p.Pgrp, &p.Session, &p.TtyNr, &p.Tpgid, &p.Flags, &p.Minflt, &p.Cminflt, &p.Majflt, &p.Cmajflt,
		&p.Utime, &p.Stime, &p.Cutime, &p.Cstime, &p.Priority, &p.Nice, &p.NumThreads, &p.ItrealValue, &p.Starttime, &p.Vsize, &p.Rss)
	if err != nil {
		log.Printf("ERROR: metrics: cannot parse %q read from %s: %s", data, statFilepath, err)
		return
	}

	// It is expensive obtaining `process_open_fds` when big number of file descriptors is opened,
	// so don't do it here.
	// See writeFDMetrics instead.

	utime := float64(p.Utime) / userHZ
	stime := float64(p.Stime) / userHZ

	// Calculate totalTime by dividing the sum of p.Utime and p.Stime by userHZ.
	// This reduces possible floating-point precision loss
	totalTime := float64(p.Utime+p.Stime) / userHZ

	WriteCounterFloat64(w, "process_cpu_seconds_system_total", stime)
	WriteCounterFloat64(w, "process_cpu_seconds_total", totalTime)
	WriteCounterFloat64(w, "process_cpu_seconds_user_total", utime)
	WriteCounterUint64(w, "process_major_pagefaults_total", uint64(p.Majflt))
	WriteCounterUint64(w, "process_minor_pagefaults_total", uint64(p.Minflt))
	WriteGaugeUint64(w, "process_num_threads", uint64(p.NumThreads))
	WriteGaugeUint64(w, "process_resident_memory_bytes", uint64(p.Rss)*pageSizeBytes)
	WriteGaugeUint64(w, "process_start_time_seconds", uint64(startTimeSeconds))
	WriteGaugeUint64(w, "process_virtual_memory_bytes", uint64(p.Vsize))
	writeProcessMemMetrics(w)
	writeIOMetrics(w)
	writePSIMetrics(w)
	writeProcessCPUThrottleMetrics(w)
}

var procSelfIOErrLogged uint32

func writeIOMetrics(w io.Writer) {
	ioFilepath := "/proc/self/io"
	data, err := os.ReadFile(ioFilepath)
	if err != nil {
		// Do not spam the logs with errors - this error cannot be fixed without process restart.
		// See https://github.com/VictoriaMetrics/metrics/issues/42
		if atomic.CompareAndSwapUint32(&procSelfIOErrLogged, 0, 1) {
			log.Printf("ERROR: metrics: cannot read process_io_* metrics from %q, so these metrics won't be updated until the error is fixed; "+
				"see https://github.com/VictoriaMetrics/metrics/issues/42 ; The error: %s", ioFilepath, err)
		}
	}

	getInt := func(s string) int64 {
		n := strings.IndexByte(s, ' ')
		if n < 0 {
			log.Printf("ERROR: metrics: cannot find whitespace in %q at %q", s, ioFilepath)
			return 0
		}
		v, err := strconv.ParseInt(s[n+1:], 10, 64)
		if err != nil {
			log.Printf("ERROR: metrics: cannot parse %q at %q: %s", s, ioFilepath, err)
			return 0
		}
		return v
	}
	var rchar, wchar, syscr, syscw, readBytes, writeBytes int64
	lines := strings.Split(string(data), "\n")
	for _, s := range lines {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, "rchar: "):
			rchar = getInt(s)
		case strings.HasPrefix(s, "wchar: "):
			wchar = getInt(s)
		case strings.HasPrefix(s, "syscr: "):
			syscr = getInt(s)
		case strings.HasPrefix(s, "syscw: "):
			syscw = getInt(s)
		case strings.HasPrefix(s, "read_bytes: "):
			readBytes = getInt(s)
		case strings.HasPrefix(s, "write_bytes: "):
			writeBytes = getInt(s)
		}
	}
	WriteGaugeUint64(w, "process_io_read_bytes_total", uint64(rchar))
	WriteGaugeUint64(w, "process_io_written_bytes_total", uint64(wchar))
	WriteGaugeUint64(w, "process_io_read_syscalls_total", uint64(syscr))
	WriteGaugeUint64(w, "process_io_write_syscalls_total", uint64(syscw))
	WriteGaugeUint64(w, "process_io_storage_read_bytes_total", uint64(readBytes))
	WriteGaugeUint64(w, "process_io_storage_written_bytes_total", uint64(writeBytes))
}

var startTimeSeconds = time.Now().Unix()

// writeFDMetrics writes process_max_fds and process_open_fds metrics to w.
func writeFDMetrics(w io.Writer) {
	totalOpenFDs, err := getOpenFDsCount("/proc/self/fd")
	if err != nil {
		log.Printf("ERROR: metrics: cannot determine open file descriptors count: %s", err)
		return
	}
	maxOpenFDs, err := getMaxFilesLimit("/proc/self/limits")
	if err != nil {
		log.Printf("ERROR: metrics: cannot determine the limit on open file descriptors: %s", err)
		return
	}
	WriteGaugeUint64(w, "process_max_fds", maxOpenFDs)
	WriteGaugeUint64(w, "process_open_fds", totalOpenFDs)
}

func getOpenFDsCount(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var totalOpenFDs uint64
	for {
		names, err := f.Readdirnames(512)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("unexpected error at Readdirnames: %s", err)
		}
		totalOpenFDs += uint64(len(names))
	}
	return totalOpenFDs, nil
}

func getMaxFilesLimit(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	const prefix = "Max open files"
	for _, s := range lines {
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		text := strings.TrimSpace(s[len(prefix):])
		// Extract soft limit.
		n := strings.IndexByte(text, ' ')
		if n < 0 {
			return 0, fmt.Errorf("cannot extract soft limit from %q", s)
		}
		text = text[:n]
		if text == "unlimited" {
			return 1<<64 - 1, nil
		}
		limit, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse soft limit from %q: %s", s, err)
		}
		return limit, nil
	}
	return 0, fmt.Errorf("cannot find max open files limit")
}

// https://man7.org/linux/man-pages/man5/procfs.5.html
type memStats struct {
	vmPeak   uint64
	rssPeak  uint64
	rssAnon  uint64
	rssFile  uint64
	rssShmem uint64
}

func writeProcessMemMetrics(w io.Writer) {
	ms, err := getMemStats("/proc/self/status")
	if err != nil {
		log.Printf("ERROR: metrics: cannot determine memory status: %s", err)
		return
	}
	WriteGaugeUint64(w, "process_virtual_memory_peak_bytes", ms.vmPeak)
	WriteGaugeUint64(w, "process_resident_memory_peak_bytes", ms.rssPeak)
	WriteGaugeUint64(w, "process_resident_memory_anon_bytes", ms.rssAnon)
	WriteGaugeUint64(w, "process_resident_memory_file_bytes", ms.rssFile)
	WriteGaugeUint64(w, "process_resident_memory_shared_bytes", ms.rssShmem)
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

// writePSIMetrics writes PSI total metrics for the current process to w.
//
// See https://docs.kernel.org/accounting/psi.html
func writePSIMetrics(w io.Writer) {
	if psiMetricsStart == nil {
		// Failed to initialize PSI metrics
		return
	}

	m, err := getPSIMetrics()
	if err != nil {
		log.Printf("ERROR: metrics: cannot expose PSI metrics: %s", err)
		return
	}

	WriteCounterFloat64(w, "process_pressure_cpu_waiting_seconds_total", psiTotalSecs(m.cpuSome-psiMetricsStart.cpuSome))
	WriteCounterFloat64(w, "process_pressure_cpu_stalled_seconds_total", psiTotalSecs(m.cpuFull-psiMetricsStart.cpuFull))

	WriteCounterFloat64(w, "process_pressure_io_waiting_seconds_total", psiTotalSecs(m.ioSome-psiMetricsStart.ioSome))
	WriteCounterFloat64(w, "process_pressure_io_stalled_seconds_total", psiTotalSecs(m.ioFull-psiMetricsStart.ioFull))

	WriteCounterFloat64(w, "process_pressure_memory_waiting_seconds_total", psiTotalSecs(m.memSome-psiMetricsStart.memSome))
	WriteCounterFloat64(w, "process_pressure_memory_stalled_seconds_total", psiTotalSecs(m.memFull-psiMetricsStart.memFull))
}

func psiTotalSecs(microsecs uint64) float64 {
	// PSI total stats is in microseconds according to https://docs.kernel.org/accounting/psi.html
	// Convert it to seconds.
	return float64(microsecs) / 1e6
}

// psiMetricsStart contains the initial PSI metric values on program start.
// it is needed in order to make sure the exposed PSI metrics start from zero.
var psiMetricsStart = func() *psiMetrics {
	m, err := getPSIMetrics()
	if err != nil {
		return nil
	}
	return m
}()

type psiMetrics struct {
	cpuSome uint64
	cpuFull uint64
	ioSome  uint64
	ioFull  uint64
	memSome uint64
	memFull uint64
}

func getPSIMetrics() (*psiMetrics, error) {
	if cgroupV2Path == "" {
		// Do nothing, since PSI requires cgroup v2, and the process doesn't run under cgroup v2.
		return nil, nil
	}

	cpuSome, cpuFull, err := readPSITotals(cgroupV2Path, "cpu.pressure")
	if err != nil {
		return nil, err
	}

	ioSome, ioFull, err := readPSITotals(cgroupV2Path, "io.pressure")
	if err != nil {
		return nil, err
	}

	memSome, memFull, err := readPSITotals(cgroupV2Path, "memory.pressure")
	if err != nil {
		return nil, err
	}

	m := &psiMetrics{
		cpuSome: cpuSome,
		cpuFull: cpuFull,
		ioSome:  ioSome,
		ioFull:  ioFull,
		memSome: memSome,
		memFull: memFull,
	}
	return m, nil
}

func readPSITotals(cgroupPath, statsName string) (uint64, uint64, error) {
	filePath := cgroupPath + "/" + statsName
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, 0, err
	}

	lines := strings.Split(string(data), "\n")
	some := uint64(0)
	full := uint64(0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "some ") && !strings.HasPrefix(line, "full ") {
			continue
		}

		tmp := strings.SplitN(line, "total=", 2)
		if len(tmp) != 2 {
			return 0, 0, fmt.Errorf("cannot find total from the line %q at %q", line, filePath)
		}
		microsecs, err := strconv.ParseUint(tmp[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot parse total=%q at %q: %w", tmp[1], filePath, err)
		}

		switch {
		case strings.HasPrefix(line, "some "):
			some = microsecs
		case strings.HasPrefix(line, "full "):
			full = microsecs
		}
	}
	return some, full, nil
}

type cgroupCPUStat struct {
	throttledNanosecs uint64
}

var cgroupCPUStatMetricsStart = func() *cgroupCPUStat {
	cpuStat, err := getCgroupCPUStats()
	if err != nil {
		return nil
	}
	return cpuStat
}()

// cgroupV2Path is the cgroup v2 directory of the process used for PSI metrics.
// cgroupCPUStatPath is the cpu.stat file of the process used for CPU throttling metrics.
// Both are empty if not available.
var cgroupV2Path, cgroupCPUStatPath = initCgroupPaths()

func initCgroupPaths() (string, string) {
	cgroupData, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", ""
	}
	var v2Mountpoint string
	for _, mp := range []string{"/sys/fs/cgroup", "/sys/fs/cgroup/unified"} {
		if isCgroup2Mount(mp) {
			v2Mountpoint = mp
			break
		}
	}
	return getCgroupPaths(string(cgroupData), v2Mountpoint)
}

func isCgroup2Mount(mountpoint string) bool {
	var st syscall.Statfs_t
	if err := syscall.Statfs(mountpoint, &st); err != nil {
		return false
	}
	// cgroup2SuperMagic is the cgroup v2 filesystem type returned by statfs.
	// See CGROUP2_SUPER_MAGIC in https://man7.org/linux/man-pages/man2/statfs.2.html
	const cgroup2SuperMagic = 0x63677270
	return st.Type == cgroup2SuperMagic
}

// getCgroupPaths returns the cgroup v2 and the cpu.stat paths for the given cgroupData
// and optional cgroup v2 mountpoint.
//
// See "Hierarchy and Controller Support" at https://systemd.io/CGROUP_DELEGATION/
// and https://github.com/opencontainers/cgroups/blob/v0.0.6/manager/new.go#L37-L54
func getCgroupPaths(cgroupData, v2Mountpoint string) (string, string) {
	var v2Path string
	if len(v2Mountpoint) > 0 {
		if rel := getCgroupV2RelativePath(cgroupData); len(rel) > 0 {
			v2Path = path.Join(v2Mountpoint, rel)
		}
	}
	if v2Mountpoint == "/sys/fs/cgroup" {
		if len(v2Path) == 0 {
			return "", ""
		}
		// fast path cgroupv2.
		return v2Path, v2Path + "/cpu.stat"
	}
	// slow path legacy or hybrid cgroup.
	var cpuPath string
	if rel := getCgroupV1RelativePath(cgroupData); len(rel) > 0 {
		// systemd and runc symlink /sys/fs/cgroup/cpu to the mountpoint of the cpu controller.
		// Also, VM lib/cgroup ignores mountpoint too:
		// https://github.com/VictoriaMetrics/VictoriaMetrics/blob/bb84352804bc4ec0619706acd1de1cb832bfc769/lib/cgroup/cpu.go#L101
		cpuPath = path.Join("/sys/fs/cgroup/cpu", rel, "/cpu.stat")
	}
	return v2Path, cpuPath
}

func getCgroupCPUStats() (*cgroupCPUStat, error) {
	if cgroupCPUStatPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(cgroupCPUStatPath)
	if err != nil {
		if os.IsNotExist(err) {
			// The cpu controller isn't enabled for the cgroup.
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read cpu.stat: %w", err)
	}
	ccs, err := parseCgroupCPUStat(string(data))
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", cgroupCPUStatPath, err)
	}
	return ccs, nil
}

func parseCgroupCPUStat(data string) (*cgroupCPUStat, error) {
	var ccs cgroupCPUStat
	lines := strings.Split(data, "\n")
	for _, s := range lines {
		field := strings.Fields(s)
		if len(field) != 2 {
			continue
		}

		fieldKey := field[0]
		fieldValue := field[1]
		value, err := strconv.ParseUint(fieldValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse number from %s: %q: %w", fieldKey, s, err)
		}
		switch fieldKey {
		case "throttled_usec":
			// In cgroup v2, the field is in microseconds.
			// See https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#cpu-interface-files
			ccs.throttledNanosecs = value * 1000
		case "throttled_time":
			// In cgroup v1, the field is in nanoseconds.
			// See https://docs.kernel.org/scheduler/sched-bwc.html#statistics
			ccs.throttledNanosecs = value
		}
	}
	return &ccs, nil
}

func writeProcessCPUThrottleMetrics(w io.Writer) {
	if cgroupCPUStatMetricsStart == nil {
		return
	}
	ccs, err := getCgroupCPUStats()
	if err != nil {
		log.Printf("ERROR: metrics: cannot determine cpu.stat: %s", err)
		return
	}
	if ccs == nil {
		return
	}
	WriteCounterFloat64(w, "process_cgroup_cpu_throttled_seconds_total", float64(ccs.throttledNanosecs-cgroupCPUStatMetricsStart.throttledNanosecs)/1e9)
}

// getCgroupV2RelativePath returns the cgroup v2 path of the process relative to
// the cgroup v2 mountpoint, or an empty string if the process doesn't run under cgroup v2.
//
// The cgroup v2 entry in /proc/self/cgroup has an empty controllers field, e.g. "0::/the/path".
// See https://man7.org/linux/man-pages/man7/cgroups.7.html
func getCgroupV2RelativePath(cgroupData string) string {
	for _, line := range strings.Split(cgroupData, "\n") {
		// Each line has the form "hierarchy-ID:controller-list:cgroup-path".
		// The cgroup v2 line has an empty hierarchy-ID and controller-list, i.e. it starts with "0::".
		tmp := strings.SplitN(line, "::", 2)
		if len(tmp) == 2 && strings.HasPrefix(line, "0::") {
			return strings.TrimSpace(tmp[1])
		}
	}
	return ""
}

func getCgroupV1RelativePath(cgroupData string) string {
	lines := strings.Split(cgroupData, "\n")
	// each line will be in the format of "hierarchy-ID:controller-list:cgroup-path"
	// e.g. 5:cpuacct,cpu,cpuset:/daemons
	// https://man7.org/linux/man-pages/man7/cgroups.7.html
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		controllers := strings.Split(parts[1], ",")
		for _, ctrl := range controllers {
			if strings.TrimSpace(ctrl) == "cpu" {
				return strings.TrimSpace(parts[2])
			}
		}
	}
	return ""
}
