// +build linux freebsd

package fs

import (
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

// MustFadviseSequentialRead hints the OS that f is read mostly sequentially.
//
// if prefetch is set, then the OS is hinted to prefetch f data.
func MustFadviseSequentialRead(f *os.File, prefetch bool) {
	fd := int(f.Fd())
	mode := unix.FADV_SEQUENTIAL
	if prefetch {
		mode |= unix.FADV_WILLNEED
	}
	if err := unix.Fadvise(int(fd), 0, 0, mode); err != nil {
		logger.Panicf("FATAL: error returned from unix.Fadvise(%d): %s", mode, err)
	}
}
