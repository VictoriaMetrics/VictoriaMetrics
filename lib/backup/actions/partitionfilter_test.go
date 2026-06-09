package actions

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/backup/common"
)

func TestPartitionFromPath(t *testing.T) {
	f := func(path, wantName string, wantIsIndexDB, wantOK bool) {
		t.Helper()
		name, isIndexDB, ok := partitionFromPath(path)
		if name != wantName || isIndexDB != wantIsIndexDB || ok != wantOK {
			t.Fatalf("partitionFromPath(%q) = (%q, %v, %v); want (%q, %v, %v)",
				path, name, isIndexDB, ok, wantName, wantIsIndexDB, wantOK)
		}
	}

	// small/big partitions
	f("data/small/2026_01/part_a/index.bin", "2026_01", false, true)
	f("data/big/2026_02/part_b/values.bin", "2026_02", false, true)

	// per-partition indexdb
	f("data/indexdb/2026_01/part_c/index.bin", "2026_01", true, true)

	// non-partition paths
	f("metadata/minTimestampForCompositeIndex", "", false, false)
	f("backup_complete.ignore", "", false, false)
	f("data/small/parts.json", "", false, false)
	// legacy global indexdb (not under data/)
	f("indexdb/177D5A697DDFB650/part_d/index.bin", "", false, false)
	// invalid partition name
	f("data/small/not_a_partition/index.bin", "", false, false)
	f("data/indexdb/177D5A697DDFB650/index.bin", "", false, false)
}

func partsFromPaths(paths ...string) []common.Part {
	parts := make([]common.Part, 0, len(paths))
	for _, p := range paths {
		parts = append(parts, common.Part{Path: p, Size: 1})
	}
	return parts
}

func pathsFromParts(parts []common.Part) []string {
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		paths = append(paths, p.Path)
	}
	return paths
}

func TestFilterPartsByPartitionsSuccess(t *testing.T) {
	parts := partsFromPaths(
		"metadata/minTimestampForCompositeIndex",
		"data/small/2026_01/part_a/index.bin",
		"data/big/2026_01/part_b/values.bin",
		"data/indexdb/2026_01/part_c/index.bin",
		"data/small/2026_02/part_d/index.bin",
		"data/indexdb/2026_02/part_e/index.bin",
		"data/small/2026_03/part_f/index.bin",
		"data/indexdb/2026_03/part_g/index.bin",
	)

	f := func(reStr string, wantPaths []string) {
		t.Helper()
		re := regexp.MustCompile(reStr)
		got, err := filterPartsByPartitions(parts, re)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		gotPaths := pathsFromParts(got)
		if !reflect.DeepEqual(gotPaths, wantPaths) {
			t.Fatalf("filterPartsByPartitions(%q) = %q; want %q", reStr, gotPaths, wantPaths)
		}
	}

	// Single partition. Non-partition metadata is always kept.
	f("^(?:2026_01)$", []string{
		"metadata/minTimestampForCompositeIndex",
		"data/small/2026_01/part_a/index.bin",
		"data/big/2026_01/part_b/values.bin",
		"data/indexdb/2026_01/part_c/index.bin",
	})

	// Multiple partitions via alternation.
	f("^(?:2026_(01|02))$", []string{
		"metadata/minTimestampForCompositeIndex",
		"data/small/2026_01/part_a/index.bin",
		"data/big/2026_01/part_b/values.bin",
		"data/indexdb/2026_01/part_c/index.bin",
		"data/small/2026_02/part_d/index.bin",
		"data/indexdb/2026_02/part_e/index.bin",
	})

	// Wildcard-like regexp.
	f("^(?:2026_.*)$", []string{
		"metadata/minTimestampForCompositeIndex",
		"data/small/2026_01/part_a/index.bin",
		"data/big/2026_01/part_b/values.bin",
		"data/indexdb/2026_01/part_c/index.bin",
		"data/small/2026_02/part_d/index.bin",
		"data/indexdb/2026_02/part_e/index.bin",
		"data/small/2026_03/part_f/index.bin",
		"data/indexdb/2026_03/part_g/index.bin",
	})
}

func TestFilterPartsByPartitionsNoMatch(t *testing.T) {
	parts := partsFromPaths(
		"metadata/minTimestampForCompositeIndex",
		"data/small/2026_01/part_a/index.bin",
		"data/indexdb/2026_01/part_c/index.bin",
	)
	re := regexp.MustCompile("^(?:2025_01)$")
	if _, err := filterPartsByPartitions(parts, re); err == nil {
		t.Fatalf("expected error when no partitions match, got nil")
	}
}

func TestFilterPartsByPartitionsMissingIndexDB(t *testing.T) {
	// Old-style backup: data partition without per-partition indexdb.
	parts := partsFromPaths(
		"metadata/minTimestampForCompositeIndex",
		"indexdb/177D5A697DDFB650/part_x/index.bin",
		"data/small/2026_01/part_a/index.bin",
		"data/big/2026_01/part_b/values.bin",
	)
	re := regexp.MustCompile("^(?:2026_01)$")
	if _, err := filterPartsByPartitions(parts, re); err == nil {
		t.Fatalf("expected error for partition without per-partition indexdb, got nil")
	}
}

func TestFilterPartsByPartitionsPartialMissingIndexDB(t *testing.T) {
	// 2026_01 has indexdb, 2026_02 doesn't. Selecting both must abort.
	parts := partsFromPaths(
		"data/small/2026_01/part_a/index.bin",
		"data/indexdb/2026_01/part_c/index.bin",
		"data/small/2026_02/part_d/index.bin",
	)
	re := regexp.MustCompile("^(?:2026_(01|02))$")
	if _, err := filterPartsByPartitions(parts, re); err == nil {
		t.Fatalf("expected error when one selected partition lacks indexdb, got nil")
	}

	// Selecting only 2026_01 must succeed.
	reOK := regexp.MustCompile("^(?:2026_01)$")
	if _, err := filterPartsByPartitions(parts, reOK); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}
