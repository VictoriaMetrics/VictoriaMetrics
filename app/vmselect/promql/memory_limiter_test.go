package promql

import (
	"testing"
)

func TestMemoryLimiter(t *testing.T) {
	var ml memoryLimiter
	ml.MaxSize = 100

	// Allocate memory
	if !ml.Get(10) {
		t.Fatalf("cannot get 10 out of %d bytes", ml.MaxSize)
	}
	if ml.usage != 10 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 10)
	}
	if !ml.Get(20) {
		t.Fatalf("cannot get 20 out of 90 bytes")
	}
	if ml.usage != 30 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 30)
	}
	if ml.Get(1000) {
		t.Fatalf("unexpected get for 1000 bytes")
	}
	if ml.usage != 30 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 30)
	}
	if ml.Get(71) {
		t.Fatalf("unexpected get for 71 bytes")
	}
	if ml.usage != 30 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 30)
	}
	if !ml.Get(70) {
		t.Fatalf("cannot get 70 bytes")
	}
	if ml.usage != 100 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 100)
	}

	// Return memory back
	ml.Put(10)
	ml.Put(70)
	if ml.usage != 20 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 20)
	}
	if !ml.Get(30) {
		t.Fatalf("cannot get 30 bytes")
	}
	ml.Put(50)
	if ml.usage != 0 {
		t.Fatalf("unexpected usage; got %d; want %d", ml.usage, 0)
	}
}
