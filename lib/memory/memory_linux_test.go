package memory

import (
	"testing"
)

func TestReadLXCMemoryLimit(t *testing.T) {
	const maxMem = 1<<31 - 1
	n, err := readLXCMemoryLimit(maxMem)
	if err != nil {
		t.Fatalf("unexpected error in readLXCMemoryLimit: %s", err)
	}
	if n < 0 {
		t.Fatalf("n must be positive; got %d", n)
	}
	if n > maxMem {
		t.Fatalf("n must be smaller than maxMem=%d; got %d", maxMem, n)
	}
}
