//go:build darwin && !ios

package metrics

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// errNotImplemented is returned by stub functions that replace cgo functions, when cgo
// isn't available.
var errNotImplemented = errors.New("not implemented")

func writeProcessMetrics(w io.Writer) {
	if memInfo, err := getMemory(); err == nil {
		WriteGaugeUint64(w, "process_resident_memory_bytes", memInfo.rss)
		WriteGaugeUint64(w, "process_virtual_memory_bytes", memInfo.vsize)
	} else if !errors.Is(err, errNotImplemented) {
		log.Printf("ERROR: metrics: %s", err)
	}

	// The proc structure returned by kern.proc.pid above has an Rusage member,
	// but it is not filled in, so it needs to be fetched by getrusage(2).  For
	// that call, the UTime, STime, and Maxrss members are filled out, but not
	// Ixrss, Idrss, or Isrss for the memory usage.  Memory stats will require
	// access to the C API to call task_info(TASK_BASIC_INFO).
	rusage := syscall.Rusage{}

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
		cpuTime := time.Duration(rusage.Stime.Nano() + rusage.Utime.Nano()).Seconds()
		WriteGaugeFloat64(w, "process_cpu_seconds_total", cpuTime)
	} else {
		log.Printf("ERROR: metrics: %s", err)
	}

	if addressSpace, err := getSoftLimit(syscall.RLIMIT_AS); err == nil {
		WriteGaugeFloat64(w, "process_virtual_memory_max_bytes", float64(addressSpace))
	} else {
		log.Printf("ERROR: metrics: %s", err)
	}
}

func writeFDMetrics(w io.Writer) {
	if fds, err := getOpenFileCount(); err == nil {
		WriteGaugeFloat64(w, "process_open_fds", fds)
	} else {
		log.Printf("ERROR: metrics: %s", err)
	}

	if openFiles, err := getSoftLimit(syscall.RLIMIT_NOFILE); err == nil {
		WriteGaugeFloat64(w, "process_max_fds", float64(openFiles))
	} else {
		log.Printf("ERROR: metrics: %s", err)
	}
}

func getOpenFileCount() (float64, error) {
	// Alternately, the undocumented proc_pidinfo(PROC_PIDLISTFDS) can be used to
	// return a list of open fds, but that requires a way to call C APIs.  The
	// benefits, however, include fewer system calls and not failing when at the
	// open file soft limit.

	if dir, err := os.Open("/dev/fd"); err != nil {
		return 0.0, err
	} else {
		defer dir.Close()

		// Avoid ReadDir(), as it calls stat(2) on each descriptor.  Not only is
		// that info not used, but KQUEUE descriptors fail stat(2), which causes
		// the whole method to fail.
		if names, err := dir.Readdirnames(0); err != nil {
			return 0.0, err
		} else {
			// Subtract 1 to ignore the open /dev/fd descriptor above.
			return float64(len(names) - 1), nil
		}
	}
}

func getSoftLimit(which int) (uint64, error) {
	rlimit := syscall.Rlimit{}

	if err := syscall.Getrlimit(which, &rlimit); err != nil {
		return 0, err
	}

	return rlimit.Cur, nil
}

func getProcessStartTime() (float64, error) {
	// Call sysctl to get kinfo_proc for current process
	mib := []int32{1 /* CTL_KERN */, 14 /* KERN_PROC */, 1 /* KERN_PROC_PID */, int32(os.Getpid())}

	// First call to get the size
	n := uintptr(0)
	_, _, errno := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		0,
		uintptr(unsafe.Pointer(&n)),
		0,
		0,
	)
	if errno != 0 {
		return 0, errno
	}
	if n == 0 {
		return 0, syscall.EINVAL
	}

	// Second call to get the actual data
	buf := make([]byte, n)
	_, _, errno = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&n)),
		0,
		0,
	)
	if errno != 0 {
		return 0, errno
	}

	// The kinfo_proc struct layout on Darwin has p_starttime (struct timeval) at specific offset
	// For amd64 and arm64, the offset is at 0x60 (96 bytes)
	// struct timeval has tv_sec (int64) and tv_usec (int32)
	const startTimeOffset = 0x60

	if len(buf) < startTimeOffset+16 {
		return 0, syscall.EINVAL
	}

	// Read tv_sec (8 bytes) and tv_usec (4 bytes)
	tvSec := int64(binary.LittleEndian.Uint64(buf[startTimeOffset:]))
	tvUsec := int32(binary.LittleEndian.Uint32(buf[startTimeOffset+8:]))

	startTime := float64(tvSec) + float64(tvUsec)/1e6
	return startTime, nil
}

type memoryInfo struct {
	vsize uint64 // Virtual memory size in bytes
	rss   uint64 // Resident memory size in bytes
}
