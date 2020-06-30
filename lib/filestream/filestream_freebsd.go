package filestream

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

func (st *streamTracker) adviseDontNeed(n int, fdatasync bool) error {
	st.length += uint64(n)
	if st.fd == 0 {
		return nil
	}
	if st.length < dontNeedBlockSize {
		return nil
	}
	blockSize := st.length - (st.length % dontNeedBlockSize)
	if fdatasync {
		if err := unixFdatasync(int(st.fd)); err != nil {
			return fmt.Errorf("unix.Fdatasync error: %w", err)
		}
	}
	if err := unix.Fadvise(int(st.fd), int64(st.offset), int64(blockSize), unix.FADV_DONTNEED); err != nil {
		return fmt.Errorf("unix.Fadvise(FADV_DONTNEEDED, %d, %d) error: %w", st.offset, blockSize, err)
	}
	st.offset += blockSize
	st.length -= blockSize
	return nil
}

func (st *streamTracker) close() error {
	if st.fd == 0 {
		return nil
	}
	// Advise the whole file as it shouldn't be cached.
	if err := unix.Fadvise(int(st.fd), 0, 0, unix.FADV_DONTNEED); err != nil {
		return fmt.Errorf("unix.Fadvise(FADV_DONTNEEDED, 0, 0) error: %w", err)
	}
	return nil
}

// unix.Fdatasync is missing, so put it here
func unixFdatasync(fd int) (err error) {
	_, _, e1 := unix.Syscall(unix.SYS_FDATASYNC, uintptr(fd), 0, 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}

func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case unix.EAGAIN:
		return syscall.EAGAIN
	case unix.EINVAL:
		return syscall.EINVAL
	case unix.ENOENT:
		return syscall.ENOENT
	}
	return e
}
