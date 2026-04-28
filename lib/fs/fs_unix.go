//go:build linux || darwin || freebsd || netbsd || openbsd

package fs

import (
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

func mmap(fd int, length int) (data []byte, err error) {
	return unix.Mmap(fd, 0, length, unix.PROT_READ, unix.MAP_SHARED)

}
func mUnmap(data []byte) error {
	return unix.Munmap(data)
}

func mustSyncPath(path string) {
	d, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot open file for fsync: %s", err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		logger.Panicf("FATAL: cannot flush %q to storage: %s", path, err)
	}
	if err := d.Close(); err != nil {
		logger.Panicf("FATAL: cannot close %q: %s", path, err)
	}
}

func createFlockFile(flockFile string) (*os.File, error) {
	flockF, err := os.Create(flockFile)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %w", flockFile, err)
	}
	if err := unix.Flock(int(flockF.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %w", flockFile, err)
	}
	return flockF, nil
}

func mustGetDiskSpace(path string) (total, free uint64) {
	var stat statfs_t
	if err := statfs(path, &stat); err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}

	total = totalSpace(stat)
	free = freeSpace(stat)
	return
}
