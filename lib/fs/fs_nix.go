//go:build linux || darwin || freebsd

package fs

import (
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

type statfs_t = unix.Statfs_t

func freeSpace(stat statfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}

func mustRemoveDirAtomic(dir string) {
	sentinelPath := filepath.Join(dir, ".delete")
	f, err := os.OpenFile(sentinelPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if !os.IsExist(err) {
			logger.Panicf("FATAL: cannot create delete sentinel %q: %s", sentinelPath, err)
		}
	} else {
		if err := f.Sync(); err != nil {
			logger.Panicf("FATAL: cannot sync delete sentinel %q: %s", sentinelPath, err)
		}
		MustClose(f)
		MustSyncPath(dir)
	}

	for _, de := range MustReadDir(dir) {
		if de.Name() == ".delete" {
			continue
		}
		MustRemoveAll(filepath.Join(dir, de.Name()))
	}

	MustRemoveAll(dir)
}

func statfs(path string, stat *statfs_t) (err error) {
	return unix.Statfs(path, stat)
}
