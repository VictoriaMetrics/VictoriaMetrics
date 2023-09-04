package storage

import (
	"os"
	"testing"
)

func TestTableOpenClose(t *testing.T) {
	const path = "TestTableOpenClose"
	const retention = 123 * retention31Days

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("cannot remove %q: %s", path, err)
	}
	defer func() {
		_ = os.RemoveAll(path)
	}()

	// Create a new table
	strg := newTestStorage()
	strg.retentionMsecs = retention.Milliseconds()
	tb := mustOpenTable(path, strg)

	// Close it
	tb.MustClose()

	// Re-open created table multiple times.
	for i := 0; i < 10; i++ {
		tb := mustOpenTable(path, strg)
		tb.MustClose()
	}

	stopTestStorage(strg)
}
