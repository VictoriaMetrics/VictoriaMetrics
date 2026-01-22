//go:build linux

package fs

import (
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func supportsMincore() bool {
	return true
}
func mincore(ptr *byte) bool {
	var result [1]byte
	_, _, err := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(ptr)), 1, uintptr(unsafe.Pointer(&result[0])))
	if err != 0 {
		logger.Panicf("FATAL: cannot call mincore(ptr=%p, 1): %s", ptr, err)
	}
	ok := (result[0] & 1) == 1
	return ok
}
