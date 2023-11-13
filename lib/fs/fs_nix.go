//go:build linux || darwin || freebsd

package fs

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

func freeSpace(stat unix.Statfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}

func mustRemoveDirAtomic(dir string) {
	n := atomic.AddUint64(&atomicDirRemoveCounter, 1)
	tmpDir := fmt.Sprintf("%s.must-remove.%d", dir, n)
	if err := os.Rename(dir, tmpDir); err != nil {
		logger.Panicf("FATAL: cannot move %s to %s: %s", dir, tmpDir, err)
	}
	MustRemoveAll(tmpDir)
}
