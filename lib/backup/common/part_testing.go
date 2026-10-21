package common

import "testing"

// SetMaxPartSizeForTest overrides MaxPartSizeForTransfer until t finishes.
//
// size must be in (0, MaxPartSize]. Use for backup/restore integration tests
// that need multi-part file layout without copying a full production part.
func SetMaxPartSizeForTest(t *testing.T, size uint64) {
	t.Helper()
	if size == 0 || size > MaxPartSize {
		t.Fatalf("SetMaxPartSizeForTest: invalid size %d; want 1..%d", size, MaxPartSize)
	}
	maxPartSizeTestOverride.Store(size)
	t.Cleanup(func() {
		maxPartSizeTestOverride.Store(0)
	})
}
