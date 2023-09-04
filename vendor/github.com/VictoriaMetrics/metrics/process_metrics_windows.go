//go:build windows
// +build windows

package metrics

import (
	"fmt"
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
	stimeSeconds := (uint64(stime.HighDateTime)<<32 + uint64(stime.LowDateTime)) / 1e7
	utimeSeconds := (uint64(utime.HighDateTime)<<32 + uint64(utime.LowDateTime)) / 1e7
	fmt.Fprintf(w, "process_cpu_seconds_system_total %d\n", stimeSeconds)
	fmt.Fprintf(w, "process_cpu_seconds_total %d\n", stimeSeconds+utimeSeconds)
	fmt.Fprintf(w, "process_cpu_seconds_user_total %d\n", stimeSeconds)
	fmt.Fprintf(w, "process_pagefaults_total %d\n", mc.PageFaultCount)
	fmt.Fprintf(w, "process_start_time_seconds %d\n", startTime.Nanoseconds()/1e9)
	fmt.Fprintf(w, "process_virtual_memory_bytes %d\n", mc.PrivateUsage)
	fmt.Fprintf(w, "process_resident_memory_peak_bytes %d\n", mc.PeakWorkingSetSize)
	fmt.Fprintf(w, "process_resident_memory_bytes %d\n", mc.WorkingSetSize)
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
	fmt.Fprintf(w, "process_max_fds %d\n", 16777216)
	fmt.Fprintf(w, "process_open_fds %d\n", count)
}
