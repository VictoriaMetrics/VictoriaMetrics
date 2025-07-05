package logstorage

import "testing"

// TestDeleteMarkerMerge verifies correct behaviour of deleteMarker.Merge.
// All test inputs and expected outputs are hard-coded following the f-test style
// already used across marker_*_test.go files.
func TestDeleteMarkerMerge(t *testing.T) {
	// build constructs a deleteMarker from a mapping blockID -> bitmap pattern string.
	build := func(blockPatterns map[uint64]string) *deleteMarker {
		dm := &deleteMarker{}
		for id, pattern := range blockPatterns {
			dm.AddBlock(id, createTestRLE(pattern))
		}
		return dm
	}

	// cmp compares deleteMarker contents against the expected mapping.
	cmp := func(dm *deleteMarker, expected map[uint64]string) bool {
		if len(dm.blockIDs) != len(expected) {
			return false
		}
		for i, id := range dm.blockIDs {
			pat, ok := expected[id]
			if !ok {
				return false
			}
			if !equalRLE(dm.rows[i], createTestRLE(pat)) {
				return false
			}
		}
		return true
	}

	f := func(a, b, expected map[uint64]string) {
		t.Helper()

		dmA := build(a)
		dmB := build(b)

		dmA.merge(dmB)

		if !cmp(dmA, expected) {
			t.Fatalf("unexpected merge result; got %+v; want %+v", dmA, expected)
		}
	}

	// 1) both markers empty
	f(map[uint64]string{}, map[uint64]string{}, map[uint64]string{})

	// 2) destination empty, source has data
	f(
		map[uint64]string{},
		map[uint64]string{3: "1"},
		map[uint64]string{3: "1"},
	)

	// 3) destination has data, source empty
	f(
		map[uint64]string{3: "1"},
		map[uint64]string{},
		map[uint64]string{3: "1"},
	)

	// 4) non-overlapping block sets
	f(
		map[uint64]string{1: "1"},
		map[uint64]string{2: "1"},
		map[uint64]string{1: "1", 2: "1"},
	)

	// 5) overlapping block that requires RLE union
	f(
		map[uint64]string{5: "1010"},
		map[uint64]string{5: "0101"},
		map[uint64]string{5: "1111"},
	)

	// 6) mix of overlapping and non-overlapping blocks
	f(
		map[uint64]string{1: "10", 3: "001"},
		map[uint64]string{1: "01", 2: "1"},
		map[uint64]string{1: "11", 2: "1", 3: "001"},
	)

	// 7) complex overlapping patterns producing full-ones result
	f(
		map[uint64]string{10: "000111000111000111"},
		map[uint64]string{10: "111000111000111000"},
		map[uint64]string{10: "111111111111111111"},
	)

	// 8) multiple blocks with various overlaps and run lengths
	f(
		map[uint64]string{
			0: "00110011",
			2: "11110000",
			4: "00001111",
			6: "10101010",
		},
		map[uint64]string{
			0: "11001100",
			1: "01010101",
			2: "00001111",
			4: "11110000",
			6: "01010101",
			7: "11111111",
		},
		map[uint64]string{
			0: "11111111",
			1: "01010101",
			2: "11111111",
			4: "11111111",
			6: "11111111",
			7: "11111111",
		},
	)
}

// TestDeleteDuringMerge verifies that rows deleted during a merge are properly handled.
func TestDeleteDuringMerge(t *testing.T) {
	// This is a placeholder for an integration test that would:
	// 1. Create a storage with multiple parts
	// 2. Start a merge operation
	// 3. Issue a delete request while the merge is in progress
	// 4. Verify that the delete is persisted as a delete marker
	// 5. Verify that after merge completion and reconciliation, the rows are properly deleted

	t.Skip("TODO: Implement integration test for delete during merge")
}
