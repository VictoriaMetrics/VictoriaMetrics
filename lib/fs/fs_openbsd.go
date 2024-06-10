package fs

import (
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/unix"
)

func freeSpace(stat unix.Statfs_t) uint64 {
	return uint64(stat.F_bavail) * uint64(stat.F_bsize)
}

func mustRemoveDirAtomic(dir string) {
	n := atomicDirRemoveCounter.Add(1)
	tmpDir := fmt.Sprintf("%s.must-remove.%d", dir, n)
	if err := os.Rename(dir, tmpDir); err != nil {
		logger.Panicf("FATAL: cannot move %s to %s: %s", dir, tmpDir, err)
	}
	MustRemoveAll(tmpDir)
}
