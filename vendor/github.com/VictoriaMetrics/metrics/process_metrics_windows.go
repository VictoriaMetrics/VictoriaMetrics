//go:build windows

package metrics

import (
	"io"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modpsapi    = syscall.NewLazyDLL("psapi.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")

	// https://learn.microsoft.com/en-us/windows/win32/api/psapi/nf-psapi-getprocessmemoryinfo
	procGetProcessMemoryInfo  = modpsapi.NewProc("GetProcessMemoryInfo")
	procGetProcessHandleCount = modkernel32.NewProc("GetProcessHandleCount")
)

// https://learn.microsoft.com/en-us/windows/win32/api/psapi/ns-psapi-process_memory_counters_ex
type processMemoryCounters struct {
	_                          uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

func writeProcessMetrics(w io.Writer) {
	h := windows.CurrentProcess()
	var startTime, exitTime, stime, utime windows.Filetime
	err := windows.GetProcessTimes(h, &startTime, &exitTime, &stime, &utime)
	if err != nil {
		log.Printf("ERROR: metrics: cannot read process times: %s", err)
		return
	}
	var mc processMemoryCounters
	r1, _, err := procGetProcessMemoryInfo.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&mc)),
		unsafe.Sizeof(mc),
	)
	if r1 != 1 {
		log.Printf("ERROR: metrics: cannot read process memory information: %s", err)
		return
	}
	stimeSeconds := float64(uint64(stime.HighDateTime)<<32+uint64(stime.LowDateTime)) / 1e7
	utimeSeconds := float64(uint64(utime.HighDateTime)<<32+uint64(utime.LowDateTime)) / 1e7
	WriteCounterFloat64(w, "process_cpu_seconds_system_total", stimeSeconds)
	WriteCounterFloat64(w, "process_cpu_seconds_total", stimeSeconds+utimeSeconds)
	WriteCounterFloat64(w, "process_cpu_seconds_user_total", stimeSeconds)
	WriteCounterUint64(w, "process_pagefaults_total", uint64(mc.PageFaultCount))
	WriteGaugeUint64(w, "process_start_time_seconds", uint64(startTime.Nanoseconds())/1e9)
	WriteGaugeUint64(w, "process_virtual_memory_bytes", uint64(mc.PrivateUsage))
	WriteGaugeUint64(w, "process_resident_memory_peak_bytes", uint64(mc.PeakWorkingSetSize))
	WriteGaugeUint64(w, "process_resident_memory_bytes", uint64(mc.WorkingSetSize))
}

func writeFDMetrics(w io.Writer) {
	h := windows.CurrentProcess()
	var count uint32
	r1, _, err := procGetProcessHandleCount.Call(
		uintptr(h),
		uintptr(unsafe.Pointer(&count)),
	)
	if r1 != 1 {
		log.Printf("ERROR: metrics: cannot determine open file descriptors count: %s", err)
		return
	}
	// it seems to be hard-coded limit for 64-bit systems
	// https://learn.microsoft.com/en-us/archive/blogs/markrussinovich/pushing-the-limits-of-windows-handles#maximum-number-of-handles
	WriteGaugeUint64(w, "process_max_fds", 16777216)
	WriteGaugeUint64(w, "process_open_fds", uint64(count))
}
