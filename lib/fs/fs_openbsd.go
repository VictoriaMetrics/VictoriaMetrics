package fs

import (
	"golang.org/x/sys/unix"
)

type statfs_t = unix.Statfs_t

func freeSpace(stat statfs_t) uint64 {
	return uint64(stat.F_bavail) * uint64(stat.F_bsize)
}

func statfs(path string, stat *statfs_t) (err error) {
	return unix.Statfs(path, stat)
}
