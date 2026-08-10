package actions

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

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
	f("metadata/tenantsMetadata.json", "")
	f("parts.json", "")
	f("backup_complete", "")
	f("", "")
}

func TestFilterPartitions(t *testing.T) {
	// Fixed reference time: 2026-06-28 00:00:00 UTC
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)

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
		makePart("data/small/2026_06/parts.json"),
		makePart("data/small/2026_06/file.bin"),
		makePart("data/big/2026_06/big.bin"),
		makePart("data/indexdb/2026_06/index.dat"),
		makePart("metadata/tenantsMetadata.json"),
	}

	f := func(restoreSince time.Duration, restorePartitionsRegexp string, wantPaths []string) {
		t.Helper()
		got, err := filterPartitions(allParts, restoreSince, restorePartitionsRegexp, now)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(got) != len(wantPaths) {
			gotPaths := make([]string, len(got))
			for i, p := range got {
				gotPaths[i] = p.Path
			}
			t.Fatalf("got %d parts %v, want %d parts %v", len(got), gotPaths, len(wantPaths), wantPaths)
		}
		for i, p := range got {
			if p.Path != wantPaths[i] {
				t.Fatalf("part[%d]: got %q, want %q", i, p.Path, wantPaths[i])
			}
		}
	}

	// No filters — all parts pass through.
	f(0, "", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01 — only Jan 2024 and metadata.
	f(0, "2024_01", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_01|2026_06 — two explicit partitions plus metadata.
	f(0, "2024_01|2026_06", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions=2024_.* — every 2024 partition (regexp, not a literal list) plus metadata.
	f(0, "2024_.*", []string{
		"data/small/2024_01/parts.json",
		"data/small/2024_01/file.bin",
		"data/big/2024_01/big.bin",
		"data/indexdb/2024_01/index.dat",
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// restoreSince=2y from 2026-06-28: cutoff is 2024-06-28.
	// 2024_01 ends 2024-02-01 — before cutoff, excluded.
	// 2024_06 ends 2024-07-01 — after cutoff (Jul 1 > Jun 28), included.
	// 2026_06 ends 2026-07-01 — after cutoff, included.
	twoYears := 2 * 365 * 24 * time.Hour
	f(twoYears, "", []string{
		"data/small/2024_06/parts.json",
		"data/small/2024_06/file.bin",
		"data/big/2024_06/big.bin",
		"data/indexdb/2024_06/index.dat",
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// Both filters: restoreSince=2y AND restorePartitions=2026_06 — only 2026_06 and metadata.
	f(twoYears, "2026_06", []string{
		"data/small/2026_06/parts.json",
		"data/small/2026_06/file.bin",
		"data/big/2026_06/big.bin",
		"data/indexdb/2026_06/index.dat",
		"metadata/tenantsMetadata.json",
	})

	// restorePartitions regexp that matches nothing in the set — only metadata survives.
	f(0, "9999_01", []string{
		"metadata/tenantsMetadata.json",
	})
}

func TestFilterPartitionsInvalidRegexp(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	parts := []common.Part{{Path: "data/small/2024_01/file.bin", FileSize: 100, Size: 100, ActualSize: 100}}

	if _, err := filterPartitions(parts, 0, "2024_0[", now); err == nil {
		t.Fatal("expected error for invalid -restorePartitions regexp '2024_0[', got nil")
	}
	if _, err := filterPartitions(parts, 0, "2024_(01", now); err == nil {
		t.Fatal("expected error for invalid -restorePartitions regexp '2024_(01', got nil")
	}
}

func TestFilterPartitionsLegacyIndexDB(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	parts := []common.Part{
		{Path: "data/small/2024_01/file.bin", FileSize: 100, Size: 100, ActualSize: 100},
		{Path: "indexdb/2024_01/index.dat", FileSize: 100, Size: 100, ActualSize: 100},
	}

	if _, err := filterPartitions(parts, 24*time.Hour, "", now); err == nil {
		t.Fatal("expected error when filtering a backup containing the legacy indexdb, got nil")
	}
	if _, err := filterPartitions(parts, 0, "2024_01", now); err == nil {
		t.Fatal("expected error when filtering a backup containing the legacy indexdb, got nil")
	}
}
