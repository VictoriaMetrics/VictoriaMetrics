package mergeset

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// LegacyCreateSnapshotAt is used for creating snapshots for legacy IndexDBs.
func (tb *Table) LegacyCreateSnapshotAt(dstDir string) error {
	logger.Infof("creating legacy IndexDB snapshot of %q...", tb.path)
	startTime := time.Now()
	if err := tb.CreateSnapshotAt(dstDir); err != nil {
		return err
	}
	logger.Infof("created legacy IndexDB snapshot of %q at %q in %.3f seconds", tb.path, dstDir, time.Since(startTime).Seconds())
	return nil
}
