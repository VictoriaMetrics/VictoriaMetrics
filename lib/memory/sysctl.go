//go:build darwin || freebsd || openbsd || dragonfly || netbsd
// +build darwin freebsd openbsd dragonfly netbsd

package memory

import (
	"syscall"
	"unsafe"
)

// This has been adapted from github.com/pbnjay/memory.
func sysctlUint64(name string) (uint64, error) {
	s, err := syscall.Sysctl(name)
	if err != nil {
		return 0, err
	}
	// hack because the string conversion above drops a \0
	b := []byte(s)
	if len(b) < 8 {
		b = append(b, 0)
	}
	return *(*uint64)(unsafe.Pointer(&b[0])), nil
}
