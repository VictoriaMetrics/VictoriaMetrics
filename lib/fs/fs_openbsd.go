package fs

import (
	"golang.org/x/sys/unix"
)

func freeSpace(stat unix.Statfs_t) uint64 {
	return uint64(stat.F_bavail) * uint64(stat.F_bsize)
}
