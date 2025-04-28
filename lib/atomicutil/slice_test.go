package atomicutil

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestSlice_NoInit(t *testing.T) {
	const workersCount = 10
	const loopsPerWorker = 100

	var s Slice[bytes.Buffer]
	bbs := s.GetSlice()
	if len(bbs) != 0 {
		t.Fatalf("unexpected length of slice: %d; want 0", len(bbs))
	}

	var wg sync.WaitGroup
	for workerID := uint(0); workerID < workersCount; workerID++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			for i := 0; i < loopsPerWorker; i++ {
				bb := s.Get(workerID)
				fmt.Fprintf(bb, "item %d at worker %d\n", i, workerID)
			}
		}(workerID)
	}
	wg.Wait()

	bbs = s.GetSlice()
	for workerID := uint(0); workerID < workersCount; workerID++ {
		var bbExpected bytes.Buffer
		for i := 0; i < loopsPerWorker; i++ {
			fmt.Fprintf(&bbExpected, "item %d at worker %d\n", i, workerID)
		}
		bb := bbs[workerID]

		result := bb.String()
		resultExpected := bbExpected.String()
		if result != resultExpected {
			t.Fatalf("unexpected result for worker %d\ngot\n%q\nwant\n%q", workerID, result, resultExpected)
		}
	}
}

func TestSlice_Init(t *testing.T) {
	const workersCount = 10
	const loopsPerWorker = 100
	const prefix = "foobar_prefix: "

	var s Slice[bytes.Buffer]
	s.Init = func(bb *bytes.Buffer) {
		bb.Write([]byte(prefix))
	}
	bbs := s.GetSlice()
	if len(bbs) != 0 {
		t.Fatalf("unexpected length of slice: %d; want 0", len(bbs))
	}

	var wg sync.WaitGroup
	for workerID := uint(0); workerID < workersCount; workerID++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			for i := 0; i < loopsPerWorker; i++ {
				bb := s.Get(workerID)
				fmt.Fprintf(bb, "item %d at worker %d\n", i, workerID)
			}
		}(workerID)
	}
	wg.Wait()

	bbs = s.GetSlice()
	for workerID := uint(0); workerID < workersCount; workerID++ {
		bbExpected := bytes.NewBufferString(prefix)
		for i := 0; i < loopsPerWorker; i++ {
			fmt.Fprintf(bbExpected, "item %d at worker %d\n", i, workerID)
		}
		bb := bbs[workerID]

		result := bb.String()
		resultExpected := bbExpected.String()
		if result != resultExpected {
			t.Fatalf("unexpected result for worker %d\ngot\n%q\nwant\n%q", workerID, result, resultExpected)
		}
	}
}
