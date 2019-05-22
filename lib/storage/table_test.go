package storage

import (
	"os"
	"testing"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	const retentionMonths = 123

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	// Create a new table
	tb, err := openTable(path, retentionMonths, nilGetDeletedMetricIDs)
	if err != nil {
		t.Fatalf("cannot create new table: %s", err)
	}

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for i := 0; i < 10; i++ {
		tb, err := openTable(path, retentionMonths, nilGetDeletedMetricIDs)
		if err != nil {
			t.Fatalf("cannot open created table: %s", err)
		}
		tb.MustClose()
	}
}

func TestTableOpenMultipleTimes(t *testing.T) {
	const path = "TestTableOpenMultipleTimes"
	const retentionMonths = 123

	defer func() {
		_ = os.RemoveAll(path)
	}()

	tb1, err := openTable(path, retentionMonths, nilGetDeletedMetricIDs)
	if err != nil {
		t.Fatalf("cannot open table the first time: %s", err)
	}
	defer tb1.MustClose()

	for i := 0; i < 10; i++ {
		tb2, err := openTable(path, retentionMonths, nilGetDeletedMetricIDs)
		if err == nil {
			tb2.MustClose()
			t.Fatalf("expecting non-nil error when opening already opened table")
		}
	}
}
