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
	bbs := s.All()
	if len(bbs) != 0 {
		t.Fatalf("unexpected length of slice: %d; want 0", len(bbs))
	}

	var wg sync.WaitGroup
	for workerID := range workersCount {
		wg.Go(func() {
			for i := 0; i < loopsPerWorker; i++ {
				bb := s.Get(uint(workerID))
				fmt.Fprintf(bb, "item %d at worker %d\n", i, workerID)
			}
		})
	}
	wg.Wait()

	bbs = s.All()
	for workerID := range workersCount {
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
	bbs := s.All()
	if len(bbs) != 0 {
		t.Fatalf("unexpected length of slice: %d; want 0", len(bbs))
	}

	var wg sync.WaitGroup
	for workerID := range workersCount {
		wg.Go(func() {
			for i := 0; i < loopsPerWorker; i++ {
				bb := s.Get(uint(workerID))
				fmt.Fprintf(bb, "item %d at worker %d\n", i, workerID)
			}
		})
	}
	wg.Wait()

	bbs = s.All()
	for workerID := range workersCount {
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
