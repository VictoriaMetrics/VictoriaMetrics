package fs

import (
	"golang.org/x/sys/unix"
)

type statfs_t = unix.Statvfs_t

func freeSpace(stat statfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}

// totalSpace returns the total capacity of the filesystem in bytes.
func totalSpace(stat statfs_t) uint64 {
	return uint64(stat.Blocks) * uint64(stat.Bsize)
}

func statfs(path string, buf *statfs_t) (err error) {
	return unix.Statvfs(path, buf)
}
