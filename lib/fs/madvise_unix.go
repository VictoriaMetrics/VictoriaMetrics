//go:build linux || darwin || freebsd || netbsd || openbsd || solaris

package fs

import "golang.org/x/sys/unix"

func madviseRandomRead(data []byte) error {
	return unix.Madvise(data, unix.MADV_RANDOM)
}
