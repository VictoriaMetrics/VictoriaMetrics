package mergeset

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// LegacyMustCreateSnapshotAt is used for creating snapshots for legacy IndexDBs.
func (tb *Table) LegacyMustCreateSnapshotAt(dstDir string) {
	logger.Infof("creating legacy IndexDB snapshot of %q...", tb.path)
	startTime := time.Now()
	tb.MustCreateSnapshotAt(dstDir)
	logger.Infof("created legacy IndexDB snapshot of %q at %q in %.3f seconds", tb.path, dstDir, time.Since(startTime).Seconds())
}
