package fslocal

import (
	"fmt"
	"os"
	"syscall"
)

const canPreallocate = true

func preallocateFile(path string, size int64) error {
	if size <= 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("cannot open %q for preallocation: %w", path, err)
	}
	err = syscall.Fallocate(int(f.Fd()), 0, 0, size)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		return fmt.Errorf("cannot fallocate %d bytes for %q: %w", size, path, err)
	}
	return nil
}
