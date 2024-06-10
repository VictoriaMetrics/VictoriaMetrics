//go:build linux || freebsd

package fs

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func fadviseSequentialRead(f *os.File, prefetch bool) error {
	fd := int(f.Fd())
	mode := unix.FADV_SEQUENTIAL
	if prefetch {
		mode |= unix.FADV_WILLNEED
	}
	if err := unix.Fadvise(int(fd), 0, 0, mode); err != nil {
		return fmt.Errorf("error returned from unix.Fadvise(%d): %w", mode, err)
	}
	return nil
}
