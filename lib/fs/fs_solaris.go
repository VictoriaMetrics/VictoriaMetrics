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
		logger.Panicf("FATAL: cannot open %q: %s", path, err)
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

	flock := unix.Flock_t{
		Type:   unix.F_WRLCK,
		Start:  0,
		Len:    0,
		Whence: 0,
	}
	if err := unix.FcntlFlock(flockF.Fd(), unix.F_SETLK, &flock); err != nil {
		return nil, fmt.Errorf("cannot acquire lock on file %q: %w", flockFile, err)
	}
	return flockF, nil
}

func mustGetFreeSpace(path string) uint64 {
	d, err := os.Open(path)
	if err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	defer MustClose(d)

	fd := d.Fd()
	var stat unix.Statvfs_t
	if err := unix.Fstatvfs(int(fd), &stat); err != nil {
		logger.Panicf("FATAL: cannot determine free disk space on %q: %s", path, err)
	}
	return freeSpace(stat)
}

func freeSpace(stat unix.Statvfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}
