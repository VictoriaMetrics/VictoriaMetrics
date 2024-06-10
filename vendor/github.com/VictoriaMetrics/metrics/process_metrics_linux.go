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
	WriteCounterFloat64(w, "process_cpu_seconds_system_total", stime)
	WriteCounterFloat64(w, "process_cpu_seconds_total", utime+stime)
	WriteCounterFloat64(w, "process_cpu_seconds_user_total", utime)
	WriteCounterUint64(w, "process_major_pagefaults_total", uint64(p.Majflt))
	WriteCounterUint64(w, "process_minor_pagefaults_total", uint64(p.Minflt))
	WriteGaugeUint64(w, "process_num_threads", uint64(p.NumThreads))
	WriteGaugeUint64(w, "process_resident_memory_bytes", uint64(p.Rss)*4096)
	WriteGaugeUint64(w, "process_start_time_seconds", uint64(startTimeSeconds))
	WriteGaugeUint64(w, "process_virtual_memory_bytes", uint64(p.Vsize))
	writeProcessMemMetrics(w)
	writeIOMetrics(w)
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
