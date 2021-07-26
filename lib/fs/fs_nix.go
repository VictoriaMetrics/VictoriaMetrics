//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package fs

import (
	"golang.org/x/sys/unix"
)

func freeSpace(stat unix.Statfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}
