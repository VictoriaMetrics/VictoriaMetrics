//go:build linux || darwin || freebsd || netbsd || openbsd || solaris

package fs

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func madviseSequentialRead(data []byte, prefetch bool) error {
	if err := unix.Madvise(data, unix.MADV_SEQUENTIAL); err != nil {
		return fmt.Errorf("error returned from unix.Madvise(%d): %w", unix.MADV_SEQUENTIAL, err)
	}
	if prefetch {
		if err := unix.Madvise(data, unix.MADV_WILLNEED); err != nil {
			return fmt.Errorf("error returned from unix.Madvise(%d): %w", unix.MADV_WILLNEED, err)
		}
	}
	return nil
}
