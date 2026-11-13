package vm

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestBlockAckWaitsForAllItems(t *testing.T) {
	im := &Importer{}
	b := im.NewBlockAck()
	for range 3 {
		b.add()
	}
	done := make(chan error, 1)
	go func() { done <- im.WaitBlock(b) }()
	select {
	case <-done:
		t.Fatal("WaitBlock returned before items finished")
	case <-time.After(10 * time.Millisecond):
	}
	b.finish(nil)
	b.finish(errors.New("import failed"))
	select {
	case <-done:
		t.Fatal("WaitBlock returned before all items finished")
	case <-time.After(10 * time.Millisecond):
	}
	b.finish(nil)
	if err := <-done; err == nil || err.Error() != "import failed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockAckIndependent(t *testing.T) {
	im := &Importer{}
	b1, b2 := im.NewBlockAck(), im.NewBlockAck()
	b1.add()
	b2.add()
	var wg sync.WaitGroup
	wg.Add(2)
	got := make(chan error, 2)
	go func() { defer wg.Done(); got <- im.WaitBlock(b1) }()
	go func() { defer wg.Done(); got <- im.WaitBlock(b2) }()
	b1.finish(nil)
	b2.finish(errors.New("block 2 failed"))
	wg.Wait()
	var failures int
	for range 2 {
		if err := <-got; err != nil {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("unexpected failures: %d", failures)
	}
}

func TestAddExtraLabelsToImportPath_Failure(t *testing.T) {
	f := func(path string, extraLabels []string) {
		t.Helper()

		_, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// bad incorrect format for extra label
	f("/api/v1/import", []string{"label=value", "bad_label_wo_value"})
}

func TestAddExtraLabelsToImportPath_Success(t *testing.T) {
	f := func(path string, extraLabels []string, resultExpected string) {
		t.Helper()

		result, err := AddExtraLabelsToImportPath(path, extraLabels)
		if err != nil {
			t.Fatalf("AddExtraLabelsToImportPath() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	// ok w/o extra labels
	f("/api/v1/import", nil, "/api/v1/import")

	// ok one extra label
	f("/api/v1/import", []string{"instance=host-1"}, "/api/v1/import?extra_label=instance%3Dhost-1")

	// ok two extra labels
	f("/api/v1/import", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?extra_label=instance%3Dhost-2&extra_label=job%3Dvmagent")

	// ok two extra with exist param
	f("/api/v1/import?timeout=50", []string{"instance=host-2", "job=vmagent"}, "/api/v1/import?timeout=50&extra_label=instance%3Dhost-2&extra_label=job%3Dvmagent")

	// ok special chars in label value
	f("/api/v1/import", []string{"team=a&b"}, "/api/v1/import?extra_label=team%3Da%26b")
}
