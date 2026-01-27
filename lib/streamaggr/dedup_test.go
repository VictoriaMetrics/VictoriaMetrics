package streamaggr

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

func TestDedupAggrSerial(t *testing.T) {
	da := newDedupAggr()

	const seriesCount = 100_000
	expectedSamplesMap := make(map[string]pushSample)
	samples := make([]pushSample, seriesCount)
	for j := range samples {
		sample := &samples[j]
		sample.key = fmt.Sprintf("key_%d", j)
		sample.value = float64(j)
		expectedSamplesMap[sample.key] = *sample
	}
	da.pushSamples(samples, 0, false)

	if n := da.sizeBytes(); n > 5_000_000 {
		t.Fatalf("too big dedupAggr state before flush: %d bytes; it shouldn't exceed 5_000_000 bytes", n)
	}
	if n := da.itemsCount(); n != seriesCount {
		t.Fatalf("unexpected itemsCount; got %d; want %d", n, seriesCount)
	}

	flushedSamplesMap := make(map[string]pushSample)
	var mu sync.Mutex
	flushSamples := func(samples []pushSample, _ int64, _ bool) {
		mu.Lock()
		for _, sample := range samples {
			flushedSamplesMap[sample.key] = sample
		}
		mu.Unlock()
	}

	flushTimestamp := time.Now().UnixMilli()
	da.flush(flushSamples, flushTimestamp, false)

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
		wg.Go(func() {
			for i := 0; i < 10; i++ {
				samples := make([]pushSample, seriesCount)
				for j := range samples {
					sample := &samples[j]
					sample.key = fmt.Sprintf("key_%d", j)
					sample.value = float64(i + j)
				}
				da.pushSamples(samples, 0, false)
			}
		})
	}
	wg.Wait()
}

func TestDeduplicateSamples(t *testing.T) {
	f := func(oldT, newT int64, oldV, newV float64, expectedT int64, expectedV float64) {
		t.Helper()
		dedupT, dedupV := deduplicateSamples(oldT, newT, oldV, newV)
		if dedupT != expectedT || dedupV != expectedV {
			t.Fatalf("unexpected deduplicated result for oldT=%d, newT=%d, oldV=%f, newV=%f; got dedupT=%d, dedupV=%f; want dedupT=%d, dedupV=%f",
				oldT, newT, oldV, newV, dedupT, dedupV, expectedT, expectedV)
		}
	}

	f(1000, 2000, 1.0, 2.0, 2000, 2.0)
	f(2000, 1000, 2.0, 1.0, 2000, 2.0)
	f(1000, 1000, 1.0, 2.0, 1000, 2.0)
	f(1000, 1000, 2.0, 1.0, 1000, 2.0)
	f(1000, 1000, 1.0, 1.0, 1000, 1.0)
	f(1000, 1000, 1.0, float64(decimal.StaleNaN), 1000, 1.0)
	f(1000, 1000, float64(decimal.StaleNaN), 2.0, 1000, 2.0)
}
