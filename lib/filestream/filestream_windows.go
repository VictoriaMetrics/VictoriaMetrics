package filestream

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func (st *streamTracker) adviseDontNeed(n int, fdatasync bool) error {
	if fdatasync && st.fd > 0 {
		if err := windows.FlushFileBuffers(windows.Handle(st.fd)); err != nil {
			return fmt.Errorf("windows.Fsync error: %w", err)
		}
	}
	return nil
}

func (st *streamTracker) close() error {
	return nil
}
