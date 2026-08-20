//go:build linux || freebsd || netbsd

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

func fadviseRandomRead(f *os.File) error {
	fd := int(f.Fd())
	if err := unix.Fadvise(fd, 0, 0, unix.FADV_RANDOM); err != nil {
		return fmt.Errorf("error returned from unix.Fadvise(FADV_RANDOM): %w", err)
	}
	return nil
}

func madviseRandomRead(data []byte) error {
	if err := unix.Madvise(data, unix.MADV_RANDOM); err != nil {
		return fmt.Errorf("error returned from unix.Madvise(MADV_RANDOM): %w", err)
	}
	return nil
}
