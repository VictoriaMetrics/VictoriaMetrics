package streamaggr

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestDedupAggrSerial(t *testing.T) {
	da := newDedupAggr()

	const seriesCount = 100_000
	expectedSamplesMap := make(map[string]pushSample)
	flushTimestamp := time.Now().UnixMilli()
	dedupTimestamp := flushTimestamp
	for i := 0; i < 2; i++ {
		windows := map[int64][]pushSample{}
		windows[flushTimestamp] = make([]pushSample, seriesCount)
		for j := range windows[flushTimestamp] {
			sample := &windows[flushTimestamp][j]
			sample.key = fmt.Sprintf("key_%d", j)
			sample.value = float64(i + j)
			expectedSamplesMap[sample.key] = *sample
		}
		da.pushSamples(windows)
	}

	if n := da.sizeBytes(); n > 4_200_000 {
		t.Fatalf("too big dedupAggr state before flush: %d bytes; it shouldn't exceed 4_200_000 bytes", n)
	}
	if n := da.itemsCount(); n != seriesCount {
		t.Fatalf("unexpected itemsCount; got %d; want %d", n, seriesCount)
	}

	flushedSamplesMap := make(map[string]pushSample)
	var mu sync.Mutex
	flushSamples := func(samples map[int64][]pushSample) {
		mu.Lock()
		for _, sample := range samples[flushTimestamp] {
			flushedSamplesMap[sample.key] = sample
		}
		mu.Unlock()
	}
	da.flush(flushSamples, dedupTimestamp, flushTimestamp)

	if !reflect.DeepEqual(expectedSamplesMap, flushedSamplesMap) {
		t.Fatalf("unexpected samples;\ngot\n%v\nwant\n%v", flushedSamplesMap, expectedSamplesMap)
	}

	if n := da.sizeBytes(); n > 17_000 {
		t.Fatalf("too big dedupAggr state after flush; %d bytes; it shouldn't exceed 17_000 bytes", n)
	}
	if n := da.itemsCount(); n != 0 {
		t.Fatalf("unexpected non-zero itemsCount after flush; got %d", n)
	}
}

func TestDedupAggrConcurrent(_ *testing.T) {
	const concurrency = 5
	const seriesCount = 10_000
	da := newDedupAggr()

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			flushTimestamp := time.Now().UnixMilli()
			for i := 0; i < 10; i++ {
				windows := map[int64][]pushSample{}
				windows[flushTimestamp] = make([]pushSample, seriesCount)
				for j := range windows[flushTimestamp] {
					sample := &windows[flushTimestamp][j]
					sample.key = fmt.Sprintf("key_%d", j)
					sample.value = float64(i + j)
				}
				da.pushSamples(windows)
			}
		}()
	}
	wg.Wait()
}
