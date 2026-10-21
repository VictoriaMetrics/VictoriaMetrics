package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

func windowedBackupRequested() bool {
	return len(*start) > 0 || len(*end) > 0 || *forceMerge
}

func applyWindowToSnapshot(ctx context.Context) error {
	if !windowedBackupRequested() {
		return nil
	}
	if *snapshotName == "" {
		return fmt.Errorf("-start/-end/-forceMerge require a local snapshot; set -snapshot.createURL or -snapshotName")
	}
	if err := snapshotutil.Validate(*snapshotName); err != nil {
		return fmt.Errorf("invalid -snapshotName=%q: %w", *snapshotName, err)
	}

	startMsec, endMsec, err := parseBackupWindow(*start, *end)
	if err != nil {
		return err
	}

	snapshotPath := filepath.Join(*storageDataPath, "snapshots", *snapshotName)
	if _, err := os.Stat(snapshotPath); err != nil {
		return fmt.Errorf("cannot open snapshot at %q: %w", snapshotPath, err)
	}
	logger.Infof("windowed backup: opening snapshot copy at %q start=%s end=%s forceMerge=%v", snapshotPath, formatWindowBound(*start, startMsec), formatWindowBound(*end, endMsec), *forceMerge)

	s := storage.MustOpenStorage(snapshotPath, storage.OpenOptions{
		DisableBackgroundMerges: true,
	})
	// MustClose is not idempotent (it closes stopCh). Use defer as the only close
	// path so panics during apply still release flock. JSON logger.Panicf calls
	// os.Exit and skips Go defers; that remains a logger limitation.
	defer func() {
		s.MustClose()
		storage.RemoveStorageRuntimeFiles(snapshotPath)
	}()
	if err := s.ApplyTimeRangeToSnapshot(ctx, startMsec, endMsec, *forceMerge); err != nil {
		return fmt.Errorf("cannot apply time window to snapshot: %w", err)
	}
	return nil
}

func parseBackupWindow(startStr, endStr string) (startMsec, endMsec int64, err error) {
	if len(startStr) == 0 && len(endStr) == 0 {
		// forceMerge-only: keep every part, then compact.
		return 0, (1<<63 - 1), nil
	}
	if len(startStr) > 0 {
		startMsec, err = timeutil.ParseTimeMsec(startStr)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot parse -start=%q: %w", startStr, err)
		}
	} else {
		// -end without -start keeps samples from the beginning of the snapshot,
		// including parts entirely before the Unix epoch.
		startMsec = math.MinInt64
	}
	if len(endStr) > 0 {
		endMsec, err = timeutil.ParseTimeMsec(endStr)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot parse -end=%q: %w", endStr, err)
		}
	} else {
		endMsec, err = timeutil.ParseTimeMsec("now")
		if err != nil {
			return 0, 0, fmt.Errorf("cannot parse default -end=now: %w", err)
		}
	}
	if startMsec >= endMsec {
		return 0, 0, fmt.Errorf("-start=%q must be less than -end=%q for a half-open window [start, end)", startStr, endStr)
	}
	return startMsec, endMsec, nil
}

func formatWindowBound(raw string, msec int64) string {
	if len(raw) == 0 {
		return fmt.Sprintf("%d", msec)
	}
	return raw
}
