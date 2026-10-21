package fslocal

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// fallocate is replaced in tests.
var fallocate = syscall.Fallocate

const canPreallocate = true

func preallocateFile(path string, size int64) error {
	if size <= 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("cannot open %q for preallocation: %w", path, err)
	}
	for {
		err = fallocate(int(f.Fd()), 0, 0, size)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	if err != nil {
		// NFS, FUSE, and some overlay mounts do not implement fallocate
		// (EOPNOTSUPP, ENOSYS, or EINVAL). Keep the sparse temporary file
		// and continue the restore. EINTR is retried above.
		if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EINVAL) {
			return nil
		}
		return fmt.Errorf("cannot fallocate %d bytes for %q: %w (try -skipFilePreallocation)", size, path, err)
	}
	return nil
}
