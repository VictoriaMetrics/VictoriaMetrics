package filestream

import (
	"fmt"

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
		if err := unix.Fsync(int(st.fd)); err != nil {
			return fmt.Errorf("unix.Fsync error: %w", err)
		}
	}
	st.offset += blockSize
	st.length -= blockSize
	return nil
}

func (st *streamTracker) close() error {
	return nil
}
