package streamaggr

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestDedupAggrSerial(t *testing.T) {
	var lc promutils.LabelsCompressor
	da := newDedupAggr(&lc)

	const seriesCount = 100_000
	expectedSamplesMap := make(map[string]pushSample)
	for i := 0; i < 2; i++ {
		samples := make([]pushSample, seriesCount)
		for j := range samples {
			sample := &samples[j]
			sample.key = fmt.Sprintf("key_%d", j)
			sample.value = float64(i + j)
			expectedSamplesMap[sample.key] = *sample
		}
		da.pushSamples(samples, 0)
	}

	if n := da.sizeBytes(); n > 6_000_000 {
		t.Fatalf("too big dedupAggr state before flush: %d bytes; it shouldn't exceed 6_000_000 bytes", n)
	}
	if n := da.itemsCount(); n != seriesCount {
		t.Fatalf("unexpected itemsCount; got %d; want %d", n, seriesCount)
	}

	flushedSamplesMap := make(map[string]pushSample)
	var mu sync.Mutex
	flushSamples := func(samples []pushSample, _ int64) {
		mu.Lock()
		for _, sample := range samples {
			flushedSamplesMap[sample.key] = sample
		}
		mu.Unlock()
	}
	da.flush(flushSamples, 0, 0)

	if !reflect.DeepEqual(expectedSamplesMap, flushedSamplesMap) {
		t.Fatalf("unexpected samples;\ngot\n%v\nwant\n%v", flushedSamplesMap, expectedSamplesMap)
	}

	da.flush(flushSamples, 1, 0)

	if n := da.sizeBytes(); n > 17_000 {
		t.Fatalf("too big dedupAggr state after flush; %d bytes; it shouldn't exceed 17_000 bytes", n)
	}
	if n := da.itemsCount(); n != 0 {
		t.Fatalf("unexpected non-zero itemsCount after flush; got %d", n)
	}
}

func TestDedupAggrConcurrent(_ *testing.T) {
	var lc promutils.LabelsCompressor
	const concurrency = 5
	const seriesCount = 10_000
	da := newDedupAggr(&lc)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				samples := make([]pushSample, seriesCount)
				for j := range samples {
					sample := &samples[j]
					sample.key = fmt.Sprintf("key_%d", j)
					sample.value = float64(i + j)
				}
				da.pushSamples(samples, 0)
			}
		}()
	}
	wg.Wait()
}
