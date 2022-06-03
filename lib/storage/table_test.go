package storage

import (
	"os"
	"testing"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	const retentionMsecs = 123 * msecsPerMonth

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	// Create a new table
	var isReadOnly uint32
	tb, err := openTable(path, nilGetDeletedMetricIDs, retentionMsecs, &isReadOnly)
	if err != nil {
		t.Fatalf("cannot create new table: %s", err)
	}

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for i := 0; i < 10; i++ {
		tb, err := openTable(path, nilGetDeletedMetricIDs, retentionMsecs, &isReadOnly)
		if err != nil {
			t.Fatalf("cannot open created table: %s", err)
		}
		tb.MustClose()
	}
}

func TestTableOpenMultipleTimes(t *testing.T) {
	const path = "TestTableOpenMultipleTimes"
	const retentionMsecs = 123 * msecsPerMonth

	defer func() {
		_ = os.RemoveAll(path)
	}()

	var isReadOnly uint32
	tb1, err := openTable(path, nilGetDeletedMetricIDs, retentionMsecs, &isReadOnly)
	if err != nil {
		t.Fatalf("cannot open table the first time: %s", err)
	}
	defer tb1.MustClose()

	for i := 0; i < 10; i++ {
		tb2, err := openTable(path, nilGetDeletedMetricIDs, retentionMsecs, &isReadOnly)
		if err == nil {
			tb2.MustClose()
			t.Fatalf("expecting non-nil error when opening already opened table")
		}
	}
}
