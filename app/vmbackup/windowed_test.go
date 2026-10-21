package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseBackupWindow(t *testing.T) {
	startMsec, endMsec, err := parseBackupWindow("now-1d", "now")
	if err != nil {
		t.Fatalf("parseBackupWindow(now-1d, now) failed: %s", err)
	}
	if startMsec > endMsec {
		t.Fatalf("start %d > end %d", startMsec, endMsec)
	}
	delta := endMsec - startMsec
	dayMsec := int64(24 * time.Hour / time.Millisecond)
	if delta < dayMsec-2000 || delta > dayMsec+2000 {
		t.Fatalf("unexpected window width %d msec; want ~%d", delta, dayMsec)
	}

	startMsec, endMsec, err = parseBackupWindow("2026-07-16T00:00:00Z", "2026-07-17T00:00:00Z")
	if err != nil {
		t.Fatalf("parseBackupWindow(RFC3339) failed: %s", err)
	}
	wantStart := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).UnixMilli()
	wantEnd := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC).UnixMilli()
	if startMsec != wantStart || endMsec != wantEnd {
		t.Fatalf("RFC3339 window got [%d, %d]; want [%d, %d]", startMsec, endMsec, wantStart, wantEnd)
	}

	unixStart := "1719792000"
	startMsec, endMsec, err = parseBackupWindow(unixStart, "now")
	if err != nil {
		t.Fatalf("parseBackupWindow(unix) failed: %s", err)
	}
	if startMsec != 1719792000*1000 {
		t.Fatalf("unix seconds parsed as %d; want %d", startMsec, 1719792000000)
	}

	if _, _, err = parseBackupWindow("now", "now-1d"); err == nil {
		t.Fatal("expected error when start > end")
	}
	if _, _, err = parseBackupWindow("2026-07-16T00:00:00Z", "2026-07-16T00:00:00Z"); err == nil {
		t.Fatal("expected error when start == end")
	}

	endOnly := "2026-07-17T00:00:00Z"
	startMsec, endMsec, err = parseBackupWindow("", endOnly)
	if err != nil {
		t.Fatalf("parseBackupWindow(empty start, %s) failed: %s", endOnly, err)
	}
	if startMsec != math.MinInt64 {
		t.Fatalf("-end without -start must start at the beginning of the snapshot; got start=%d", startMsec)
	}
	if endMsec != wantEnd {
		t.Fatalf("-end without -start got end=%d; want %d", endMsec, wantEnd)
	}

	startMsec, endMsec, err = parseBackupWindow("", "now")
	if err != nil {
		t.Fatalf("parseBackupWindow(empty start, now) failed: %s", err)
	}
	if startMsec != math.MinInt64 || endMsec <= 0 {
		t.Fatalf("-end=now without -start got [%d, %d]; want start=MinInt64 and end>0", startMsec, endMsec)
	}

	startMsec, endMsec, err = parseBackupWindow("", "")
	if err != nil {
		t.Fatalf("forceMerge-only window failed: %s", err)
	}
	if startMsec != 0 || endMsec != (1<<63-1) {
		t.Fatalf("forceMerge-only window got [%d, %d]", startMsec, endMsec)
	}
}

func TestWindowedBackupRequested(t *testing.T) {
	origStart, origEnd, origForce := *start, *end, *forceMerge
	t.Cleanup(func() {
		*start = origStart
		*end = origEnd
		*forceMerge = origForce
	})

	*start, *end, *forceMerge = "", "", false
	if windowedBackupRequested() {
		t.Fatal("unset flags must keep current full-backup path")
	}

	*start = "now-1d"
	if !windowedBackupRequested() {
		t.Fatal("expected windowed backup when -start is set")
	}

	*start, *end = "", "now"
	if !windowedBackupRequested() {
		t.Fatal("expected windowed backup when only -end is set")
	}

	*start, *end, *forceMerge = "", "", true
	if !windowedBackupRequested() {
		t.Fatal("expected windowed backup when only -forceMerge is set")
	}
}

func TestMakeBackupRejectsWindowFlagsWithoutSnapshot(t *testing.T) {
	origName, origStart, origEnd, origForce := *snapshotName, *start, *end, *forceMerge
	t.Cleanup(func() {
		*snapshotName = origName
		*start = origStart
		*end = origEnd
		*forceMerge = origForce
	})

	*snapshotName = ""
	cases := []struct {
		name  string
		start string
		end   string
		force bool
	}{
		{name: "start", start: "now-1d"},
		{name: "end", end: "now"},
		{name: "forceMerge", force: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			*start, *end, *forceMerge = tc.start, tc.end, tc.force
			err := makeBackup(context.Background())
			if err == nil {
				t.Fatal("expected error when window flags are set without a snapshot")
			}
			if !strings.Contains(err.Error(), "require a local snapshot") {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestApplyWindowToSnapshotRejectsInvalidName(t *testing.T) {
	origName, origPath, origStart, origEnd := *snapshotName, *storageDataPath, *start, *end
	t.Cleanup(func() {
		*snapshotName = origName
		*storageDataPath = origPath
		*start = origStart
		*end = origEnd
	})

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "snapshots"), 0o700); err != nil {
		t.Fatalf("mkdir snapshots: %s", err)
	}
	marker := filepath.Join(dir, "keep-me")
	if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write marker: %s", err)
	}

	*storageDataPath = dir
	*snapshotName = ".."
	*start = "now-1d"
	*end = "now"
	err := applyWindowToSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected error for -snapshotName=..")
	}
	if !strings.Contains(err.Error(), "invalid -snapshotName") {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("live data path was mutated: marker missing: %s", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "flock.lock")); !os.IsNotExist(statErr) {
		t.Fatal("MustOpenStorage ran on live data path; flock.lock exists")
	}
}
