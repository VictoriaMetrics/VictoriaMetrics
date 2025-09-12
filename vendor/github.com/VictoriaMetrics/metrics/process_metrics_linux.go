package metrics

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
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
	data, err := ioutil.ReadFile(statFilepath)
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
}

var procSelfIOErrLogged uint32

func writeIOMetrics(w io.Writer) {
	ioFilepath := "/proc/self/io"
	data, err := ioutil.ReadFile(ioFilepath)
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
		log.Printf("ERROR: metrics: cannot determine the limit on open file descritors: %s", err)
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
	data, err := ioutil.ReadFile(path)
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
	data, err := ioutil.ReadFile(path)
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
		log.Printf("INFO: metrics: disable exposing PSI metrics because of failed init: %s", err)
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
	cgroupPath := getCgroupV2Path()
	if cgroupPath == "" {
		// Do nothing, since PSI requires cgroup v2, and the process doesn't run under cgroup v2.
		return nil, nil
	}

	cpuSome, cpuFull, err := readPSITotals(cgroupPath, "cpu.pressure")
	if err != nil {
		return nil, err
	}

	ioSome, ioFull, err := readPSITotals(cgroupPath, "io.pressure")
	if err != nil {
		return nil, err
	}

	memSome, memFull, err := readPSITotals(cgroupPath, "memory.pressure")
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
	data, err := ioutil.ReadFile(filePath)
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

func getCgroupV2Path() string {
	data, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	tmp := strings.SplitN(string(data), "::", 2)
	if len(tmp) != 2 {
		return ""
	}
	path := "/sys/fs/cgroup" + strings.TrimSpace(tmp[1])

	// Drop trailing slash if it exsits. This prevents from '//' in the constructed paths by the caller.
	return strings.TrimSuffix(path, "/")
}
