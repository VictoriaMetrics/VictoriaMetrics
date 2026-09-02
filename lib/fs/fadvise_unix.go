//go:build linux || freebsd || netbsd

package fs

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func fadviseSequentialRead(f *os.File, prefetch bool) error {
	fd := int(f.Fd())
	if err := unix.Fadvise(fd, 0, 0, unix.FADV_SEQUENTIAL); err != nil {
		return fmt.Errorf("error returned from unix.Fadvise(%d): %w", unix.FADV_SEQUENTIAL, err)
	}
	if prefetch {
		if err := unix.Fadvise(fd, 0, 0, unix.FADV_WILLNEED); err != nil {
			return fmt.Errorf("error returned from unix.Fadvise(%d): %w", unix.FADV_WILLNEED, err)
		}
	}
	return nil
}
