package netutil

import (
	"golang.org/x/sys/unix"
	"syscall"
	"time"
)

func setTCPUserTimeout(fd uintptr, timeout time.Duration) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(timeout.Milliseconds()))
}
