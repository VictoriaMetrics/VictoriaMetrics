package mergeset

import (
	"context"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// LegacyMustCreateSnapshotAt is used for creating snapshots for legacy IndexDBs.
func (tb *Table) LegacyMustCreateSnapshotAt(dstDir string) {
	if err := tb.LegacyCreateSnapshotAt(context.Background(), dstDir); err != nil {
		logger.Panicf("BUG: cannot create legacy IndexDB snapshot: %s", err)
	}
}

// LegacyCreateSnapshotAt creates a snapshot of a legacy IndexDB in dstDir.
func (tb *Table) LegacyCreateSnapshotAt(ctx context.Context, dstDir string) error {
	logger.Infof("creating legacy IndexDB snapshot of %q...", tb.path)
	startTime := time.Now()
	if err := tb.CreateSnapshotAt(ctx, dstDir); err != nil {
		return err
	}
	logger.Infof("created legacy IndexDB snapshot of %q at %q in %.3f seconds", tb.path, dstDir, time.Since(startTime).Seconds())
	return nil
}
