//go:build !linux

package netutil

import (
	"time"
)

func setTCPUserTimeout(fd uintptr, timeout time.Duration) error {
	return nil
}
