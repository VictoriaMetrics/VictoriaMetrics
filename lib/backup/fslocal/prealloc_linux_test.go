package fslocal

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestPreallocateFileUnsupportedContinues(t *testing.T) {
	for _, errno := range []error{syscall.EOPNOTSUPP, syscall.ENOSYS, syscall.EINVAL} {
		t.Run(errno.Error(), func(t *testing.T) {
			orig := fallocate
			t.Cleanup(func() { fallocate = orig })
			fallocate = func(_ int, _ uint32, _, _ int64) error {
				return errno
			}
			path := filepath.Join(t.TempDir(), "f")
			if err := preallocateFile(path, 4096); err != nil {
				t.Fatalf("fallocate %s: %s", errno, err)
			}
		})
	}
}

func TestPreallocateFileRetriesEINTR(t *testing.T) {
	orig := fallocate
	t.Cleanup(func() { fallocate = orig })
	calls := 0
	fallocate = func(_ int, _ uint32, _, _ int64) error {
		calls++
		if calls < 3 {
			return syscall.EINTR
		}
		return nil
	}
	path := filepath.Join(t.TempDir(), "f")
	if err := preallocateFile(path, 4096); err != nil {
		t.Fatalf("fallocate EINTR retry: %s", err)
	}
	if calls != 3 {
		t.Fatalf("fallocate calls = %d; want 3", calls)
	}
}

func TestPreallocateFileOtherErrorMentionsSkipFlag(t *testing.T) {
	orig := fallocate
	t.Cleanup(func() { fallocate = orig })
	fallocate = func(_ int, _ uint32, _, _ int64) error {
		return syscall.ENOSPC
	}
	path := filepath.Join(t.TempDir(), "f")
	err := preallocateFile(path, 4096)
	if err == nil {
		t.Fatal("expected fallocate error")
	}
	if !strings.Contains(err.Error(), "-skipFilePreallocation") {
		t.Fatalf("error %q does not mention -skipFilePreallocation", err)
	}
}
