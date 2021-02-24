package fs

import (
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

func mustGetFreeSpace(path string) uint64 {
	d, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	defer MustClose(d)

	fd := d.Fd()
	var stat unix.Statfs_t
	if err := unix.Fstatfs(int(fd), &stat); err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	return freeSpace(stat)
}

func freeSpace(stat unix.Statfs_t) uint64 {
	return uint64(stat.F_bavail) * uint64(stat.F_bsize)
}
