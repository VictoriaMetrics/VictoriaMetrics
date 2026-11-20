package actions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/backupnames"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fslocal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/fsremote"
)

func TestRestoreBackupObjects(t *testing.T) {
	data := bytes.Repeat([]byte("vmrestore"), 128*1024)
	p := common.Part{Path: "data/big/2026_11/file", FileSize: uint64(len(data)), Size: uint64(len(data)), ActualSize: uint64(len(data))}
	src := &fsremote.FS{Dir: t.TempDir()}
	if err := src.UploadPart(p, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	empty := common.Part{Path: "metadata/empty"}
	if err := src.UploadPart(empty, bytes.NewReader(nil)); err != nil {
		t.Fatal(err)
	}
	if err := src.CreateFile(backupnames.BackupCompleteFilename, nil); err != nil {
		t.Fatal(err)
	}
	r := &Restore{
		Src:         src,
		Dst:         &fslocal.FS{Dir: t.TempDir()},
		Concurrency: 10,
	}
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("cannot restore original backup object: %s", err)
	}
	f, err := os.Open(p.LocalPath(r.Dst.Dir))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(data)
	if n != int64(len(data)) || !bytes.Equal(h.Sum(nil), wantHash[:]) {
		t.Fatal("restored file differs from backup")
	}
	fi, err := os.Stat(empty.LocalPath(r.Dst.Dir))
	if err != nil || fi.Size() != 0 {
		t.Fatalf("empty file wasn't restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.Dst.Dir, backupnames.RestoreInProgressFilename)); !os.IsNotExist(err) {
		t.Fatalf("incomplete restore marker remains after success: %v", err)
	}
}

func TestRestoreDownloadFailureAndResume(t *testing.T) {
	data := []byte("restored data")
	p := common.Part{Path: "data/file", FileSize: uint64(len(data)), Size: uint64(len(data)), ActualSize: uint64(len(data))}
	src := &concurrencyTrackingRemoteFS{parts: []common.Part{p}, data: map[common.Part][]byte{p: data}}
	downloadErr := errors.New("interrupted download")
	src.downloadPart = func(_ common.Part, w io.Writer) error {
		if _, err := w.Write(data[:3]); err != nil {
			return err
		}
		return downloadErr
	}
	r := &Restore{
		Src:                     src,
		Dst:                     &fslocal.FS{Dir: t.TempDir()},
		Concurrency:             2,
		SkipBackupCompleteCheck: true,
	}
	if err := r.Run(context.Background()); !errors.Is(err, downloadErr) {
		t.Fatalf("unexpected restore error: %v", err)
	}
	if _, err := os.Stat(p.LocalPath(r.Dst.Dir)); !os.IsNotExist(err) {
		t.Fatalf("failed download was finalized: %v", err)
	}
	lockPath := filepath.Join(r.Dst.Dir, backupnames.RestoreInProgressFilename)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("missing incomplete restore marker: %s", err)
	}
	src.downloadPart = nil
	if err := r.Run(context.Background()); err != nil {
		t.Fatalf("cannot resume failed restore: %s", err)
	}
	got, err := os.ReadFile(p.LocalPath(r.Dst.Dir))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("unexpected resumed file: %q; err=%v", got, err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("incomplete restore marker remains after success: %v", err)
	}
}

func TestExtractPartitionName(t *testing.T) {
	f := func(partPath, expectedName string) {
		t.Helper()
		got := extractPartitionName(partPath)
		if got != expectedName {
			t.Fatalf("extractPartitionName(%q): got %q, want %q", partPath, got, expectedName)
		}
	}

	// Small partition files.
	f("data/small/2024_01/parts.json", "2024_01")
	f("data/small/2024_01/some/nested/file.bin", "2024_01")

	// Big partition files.
	f("data/big/2026_06/parts.json", "2026_06")

	// IndexDB partition files.
	f("data/indexdb/2023_12/index.dat", "2023_12")

	// Non-partition paths — expect empty string.
	f("data/small/file.bin", "")
	f("data/small/snapshots/file.bin", "")
	f("indexdb/1838A2E6BE5C4E9F/parts.json", "")
	f("metadata/tenantsMetadata.json", "")
	f("parts.json", "")
	f("backup_complete", "")
	f("", "")
}

func TestFilterPartitions(t *testing.T) {
	// Fixed reference time: 2026-06-28 00:00:00 UTC
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)

	f := func(restoreSince time.Duration, restorePartitionsRegexp string, wantPaths []string) {
		t.Helper()

		makePart := func(path string) common.Part {
			return common.Part{Path: path, FileSize: 100, Size: 100, ActualSize: 100}
		}

		allParts := []common.Part{
			makePart("data/small/2024_01/parts.json"),
			makePart("data/small/2024_01/file.bin"),
			makePart("data/big/2024_01/big.bin"),
			makePart("data/indexdb/2024_01/index.dat"),
			makePart("data/small/2024_06/parts.json"),
			makePart("data/small/2024_06/file.bin"),
			makePart("data/big/2024_06/big.bin"),
			makePart("data/indexdb/2024_06/index.dat"),
			makePart("data/small/2025_12/file.bin"),
			makePart("data/small/2026_06/parts.json"),
			makePart("data/small/2026_06/file.bin"),
			makePart("data/big/2026_06/big.bin"),
			makePart("data/indexdb/2026_06/index.dat"),
			makePart("data/big/snapshots/data.bin"),
			makePart("indexdb/1838A2E6BE5C4E9F/parts.json"),
			makePart("metadata/tenantsMetadata.json"),
		}

		var partitionRe *regexp.Regexp
		if restorePartitionsRegexp != "" {
			partitionRe = regexp.MustCompile(restorePartitionsRegexp)
		}
		got, err := filterPartitions(allParts, restoreSince, partitionRe, now)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		gotPaths := make([]string, len(got))
		for i, p := range got {
			gotPaths[i] = p.Path
		}
		if !reflect.DeepEqual(gotPaths, wantPaths) {
			t.Fatalf("unexpected parts\ngot\n%v\nwant\n%v", gotPaths, wantPaths)
		}
	}

	// No filters — all parts pass through except the legacy indexdb.
	f(0, "", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01 — only Jan 2024 plus non-partition files.
	f(0, "2024_01", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01|2026_06 — two explicit partitions plus non-partition files.
	f(0, "2024_01|2026_06", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_.* — every 2024 partition (regexp, not a literal list) plus non-partition files.
	f(0, "2024_.*", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// restoreSince=2y from 2026-06-28: cutoff is 2024-06-28.
	// 2024_01 ends 2024-02-01 — before cutoff, excluded.
	// 2024_06 ends 2024-07-01 — after cutoff (Jul 1 > Jun 28), included.
	// 2025_12 ends 2026-01-01 — after cutoff, included.
	// 2026_06 ends 2026-07-01 — after cutoff, included.
	twoYears := 2 * 365 * 24 * time.Hour
	f(twoYears, "", []string{
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// December rollover: 2025_12 ends 2026-01-01 (time.Date normalizes month 13)
	// Cutoff 2025-12-31 23:00 — 2025_12 included.
	f(now.Sub(time.Date(2025, 12, 31, 23, 0, 0, 0, time.UTC)), "", []string{
		"data/small/2025_12/file.bin",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})
	// Cutoff exactly 2026-01-01 — 2025_12 excluded, since the partition end is exclusive.
	f(now.Sub(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), "", []string{
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})

	// Both filters: restoreSince=2y AND restorePartitions=2026_06 — only 2026_06 plus non-partition files.
	f(twoYears, "2026_06", []string{
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"data/big/snapshots/data.bin",
		"metadata/tenantsMetadata.json",
	})
}

func TestFilterPartitionsNothingSelected(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	parts := []common.Part{
		{Path: "data/small/2024_01/file.bin", FileSize: 100, Size: 100, ActualSize: 100},
		{Path: "data/small/2024_02/file.bin", FileSize: 100, Size: 100, ActualSize: 100},
		{Path: "metadata/tenantsMetadata.json", FileSize: 100, Size: 100, ActualSize: 100},
	}

	_, err := filterPartitions(parts, 0, regexp.MustCompile("9999_01"), now)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	errExpected := "-restoreSince and -restorePartitions don't select any of 2 partitions in the backup; check the flag values"
	if err.Error() != errExpected {
		t.Fatalf("unexpected error\ngot\n%s\nwant\n%s", err, errExpected)
	}
}
