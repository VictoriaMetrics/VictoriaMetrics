package promutil

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestLabelsCompressorSerial(t *testing.T) {
	var lc LabelsCompressor

	f := func(labels []prompb.Label) {
		t.Helper()

		sExpected := labelsToString(labels)

		data := lc.Compress(nil, labels)
		labelsResult := lc.Decompress(nil, data)

		sResult := labelsToString(labelsResult)
		if sExpected != sResult {
			t.Fatalf("unexpected result; got %s; want %s", sResult, sExpected)
		}

		if len(labels) > 0 {
			if n := lc.SizeBytes(); n == 0 {
				t.Fatalf("Unexpected zero SizeBytes()")
			}
			if n := lc.ItemsCount(); n == 0 {
				t.Fatalf("Unexpected zero ItemsCount()")
			}
		}
	}

	// empty labels
	f(nil)
	f([]prompb.Label{})

	// non-empty labels
	f([]prompb.Label{
		{
			Name:  "instance",
			Value: "12345.4342.342.3",
		},
		{
			Name:  "job",
			Value: "kube-pod-12323",
		},
	})
	f([]prompb.Label{
		{
			Name:  "instance",
			Value: "12345.4342.342.3",
		},
		{
			Name:  "job",
			Value: "kube-pod-12323",
		},
		{
			Name:  "pod",
			Value: "foo-bar-baz",
		},
	})
}

func TestLabelsCompressorConcurrent(t *testing.T) {
	const concurrency = 5
	var lc LabelsCompressor
	var expectCompressedKeys sync.Map

	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Go(func() {
			series := newTestSeries(100, 20)
			for n, labels := range series {
				sExpected := labelsToString(labels)
				data := lc.Compress(nil, labels)
				if expectData, ok := expectCompressedKeys.LoadOrStore(n, data); ok {
					if string(data) != string(expectData.([]byte)) {
						panic(fmt.Errorf("unexpected compress result at series/%d in iteration %d ", n, i))
					}
				}
				labelsResult := lc.Decompress(nil, data)
				sResult := labelsToString(labelsResult)
				if sExpected != sResult {
					panic(fmt.Errorf("unexpected result on iteration %d; got %s; want %s", i, sResult, sExpected))
				}
			}
		})
	}
	wg.Wait()

	if n := lc.SizeBytes(); n == 0 {
		t.Fatalf("Unexpected zero SizeBytes()")
	}
	if n := lc.ItemsCount(); n == 0 {
		t.Fatalf("Unexpected zero ItemsCount()")
	}
}

func labelsToString(labels []prompb.Label) string {
	l := Labels{
		Labels: labels,
	}
	return l.String()
}

func newTestSeries(seriesCount, labelsPerSeries int) [][]prompb.Label {
	series := make([][]prompb.Label, seriesCount)
	for i := range seriesCount {
		labels := make([]prompb.Label, labelsPerSeries)
		for j := range labelsPerSeries {
			labels[j] = prompb.Label{
				Name:  fmt.Sprintf("label_%d", j),
				Value: fmt.Sprintf("value_%d_%d", i, j),
			}
		}
		series[i] = labels
	}
	return series
}

func TestLabelsCompressorCleanupFreesUnreferencedLabels(t *testing.T) {
	var lc LabelsCompressor
	for _, labels := range newTestSeries(1000, 10) {
		lc.Compress(nil, labels)
	}
	itemsBefore, sizeBefore := lc.ItemsCount(), lc.SizeBytes()
	if itemsBefore == 0 {
		t.Fatalf("expected items after compressing")
	}

	// The first Cleanup keeps the labels, the second moves them to standby and the third frees them.
	for i := 1; i <= 2; i++ {
		if freed := lc.Cleanup(time.Now(), time.Now); freed != 0 {
			t.Fatalf("Cleanup #%d freed %d labels; want 0", i, freed)
		}
	}
	if freed := lc.Cleanup(time.Now(), time.Now); freed != int(itemsBefore) {
		t.Fatalf("third Cleanup freed %d labels; want %d", freed, itemsBefore)
	}
	if n := lc.ItemsCount(); n != 0 {
		t.Fatalf("ItemsCount after freeing everything = %d; want 0", n)
	}
	if lc.SizeBytes() >= sizeBefore {
		t.Fatalf("SizeBytes did not shrink: before=%d after=%d", sizeBefore, lc.SizeBytes())
	}

	// New labels reuse the freed idxs and must decode correctly.
	nextIdxBefore := lc.nextIdx
	for _, labels := range newTestSeries(100, 10) {
		labels = append(labels, prompb.Label{Name: "generation", Value: "2"})
		got := lc.Decompress(nil, lc.Compress(nil, labels))
		if labelsToString(got) != labelsToString(labels) {
			t.Fatalf("unexpected labels after reuse; got %s; want %s", labelsToString(got), labelsToString(labels))
		}
	}
	if lc.nextIdx != nextIdxBefore {
		t.Fatalf("new labels allocated fresh idxs (%d -> %d) instead of reusing freed ones", nextIdxBefore, lc.nextIdx)
	}
}

func TestLabelsCompressorCleanupKeepsKeysOfReferencedLabels(t *testing.T) {
	var lc LabelsCompressor
	labels := []prompb.Label{{Name: "job", Value: "api"}, {Name: "instance", Value: "10.0.0.1:9100"}}
	key := lc.Compress(nil, labels)
	for i := range 10 {
		lc.Cleanup(time.Now(), time.Now)
		if got := lc.Compress(nil, labels); !bytes.Equal(got, key) {
			t.Fatalf("key of a label used every period changed after %d Cleanups", i+1)
		}
	}
}

func TestLabelsCompressorCleanupRestoresIdxOfStandbyLabel(t *testing.T) {
	var lc LabelsCompressor
	labels := []prompb.Label{{Name: "pod", Value: "api-7d9f-x2"}}
	key := lc.Compress(nil, labels)

	// Not used for one Cleanup, so it is on standby. Its key must not change.
	lc.Cleanup(time.Now(), time.Now)
	lc.Cleanup(time.Now(), time.Now)
	if got := lc.Compress(nil, labels); !bytes.Equal(got, key) {
		t.Fatalf("label compressed again after one unreferenced period got a different key")
	}
	if got := lc.Decompress(nil, key); labelsToString(got) != labelsToString(labels) {
		t.Fatalf("unexpected labels; got %s; want %s", labelsToString(got), labelsToString(labels))
	}
}

func TestLabelsCompressorCleanupKeepsLabelsCompressedDuringScan(t *testing.T) {
	var lc LabelsCompressor
	labels := []prompb.Label{{Name: "pod", Value: "api-7d9f-x2"}}
	lc.Compress(nil, labels)
	lc.Cleanup(time.Now(), time.Now)

	// A Compress that loads the label during Cleanup and marks it after Cleanup returns.
	// Cleanup calls now after the scan, so the label is on standby by then and Compress restores it.
	// If Cleanup called now before the scan, the label would be freed while the key still uses it.
	stamp := time.Now()
	var e *labelEntry
	lc.Cleanup(stamp, func() time.Time {
		if v, ok := lc.labelToIdx.Load(labels[0]); ok {
			e = v.(*labelEntry)
		}
		return stamp
	})
	var key []byte
	if e != nil {
		e.referenced.Store(true)
		key = encoding.MarshalVarUint64s(nil, []uint64{1, e.idx})
	} else {
		key = lc.Compress(nil, labels)
	}

	// The key was compressed after now returned.
	lc.Cleanup(stamp.Add(time.Millisecond), time.Now)
	if got := lc.Decompress(nil, key); labelsToString(got) != labelsToString(labels) {
		t.Fatalf("unexpected labels; got %s; want %s", labelsToString(got), labelsToString(labels))
	}
}

func TestLabelsCompressorCleanupKeepsHeldKeys(t *testing.T) {
	var lc LabelsCompressor
	const workers = 16
	// Each worker stores the oldest start time of the keys it holds, so keysGoneBefore
	// never passes a key that is still in use.
	var inUse sync.RWMutex
	pinnedStarts := make([]atomic.Int64, workers)
	flappingStarts := make([]atomic.Int64, workers)
	var freed atomic.Int64
	stop := make(chan struct{})
	var cleanups sync.WaitGroup
	cleanups.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			inUse.Lock()
			keysGoneBefore := time.Now().UnixNano()
			for i := range workers {
				keysGoneBefore = min(keysGoneBefore, pinnedStarts[i].Load(), flappingStarts[i].Load())
			}
			inUse.Unlock()
			freed.Add(int64(lc.Cleanup(time.Unix(0, keysGoneBefore), time.Now)))
		}
	})

	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			pinned := []prompb.Label{{Name: "pinned", Value: fmt.Sprint(w)}}
			flapping := []prompb.Label{{Name: "flapping", Value: fmt.Sprint(w)}}
			var pinnedKey, flappingKey []byte
			for iter := range 4_000 {
				inUse.RLock()
				start := time.Now().UnixNano()
				pinnedStarts[w].Store(start)
				key := lc.Compress(nil, pinned)
				if iter%64 == 0 {
					flappingStarts[w].Store(start)
					newFlappingKey := lc.Compress(nil, flapping)
					if flappingKey != nil && !bytes.Equal(newFlappingKey, flappingKey) {
						t.Errorf("worker %d: key of a held label changed at iteration %d", w, iter)
					}
					flappingKey = newFlappingKey
				}
				churn := []prompb.Label{{Name: "id", Value: fmt.Sprintf("w%d-%d", w, iter)}}
				churnKey := lc.Compress(nil, churn)
				inUse.RUnlock()

				if pinnedKey != nil && !bytes.Equal(key, pinnedKey) {
					t.Errorf("worker %d: key of a label used every iteration changed at iteration %d", w, iter)
					return
				}
				pinnedKey = key
				for _, c := range []struct {
					key    []byte
					labels []prompb.Label
				}{{pinnedKey, pinned}, {flappingKey, flapping}, {churnKey, churn}} {
					if got := lc.Decompress(nil, c.key); labelsToString(got) != labelsToString(c.labels) {
						t.Errorf("worker %d: decoded %s; want %s", w, labelsToString(got), labelsToString(c.labels))
						return
					}
				}
			}
		})
	}
	wg.Wait()
	close(stop)
	cleanups.Wait()
	if freed.Load() == 0 {
		t.Fatalf("Cleanup freed no labels; the test did not exercise eviction")
	}
}
