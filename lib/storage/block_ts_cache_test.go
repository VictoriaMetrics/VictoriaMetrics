package storage

import (
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

func jitterTimestamps(seed int64, rows int) []int64 {
	rng := rand.New(rand.NewSource(seed))
	a := make([]int64, rows)
	for i := range a {
		a[i] = 1_700_000_000_000 + int64(i)*15000 + int64(rng.Intn(21))
	}
	return a
}

// loadBlock marshals timestamps and unmarshals them into b with the given cache.
func loadBlock(t *testing.T, b *Block, c *TimestampsCache, timestamps []int64) {
	t.Helper()
	var src Block
	src.Init(&TSID{}, timestamps, make([]int64, len(timestamps)), 0, 64)
	src.MarshalData(0, 0)
	b.Reset()
	if _, err := UnmarshalBlock(b, MarshalBlock(nil, &src)); err != nil {
		t.Fatalf("UnmarshalBlock: %s", err)
	}
	if err := b.UnmarshalDataCached(c); err != nil {
		t.Fatalf("UnmarshalDataCached: %s", err)
	}
}

func TestTimestampsCacheLimit(t *testing.T) {
	SetTimestampsCacheMaxBytes(1 << 20)
	defer SetTimestampsCacheMaxBytes(0)

	var c1, c2 TimestampsCache
	var b Block
	a := jitterTimestamps(1, 4096)
	loadBlock(t, &b, &c1, a)
	if n := tsCacheBytes.Load(); n != tsCacheSeenBytes || n != c1.bytes.Load() {
		t.Fatalf("first sighting must store the marker only; got total=%d, c1=%d", n, c1.bytes.Load())
	}
	loadBlock(t, &b, &c1, a)
	n := tsCacheBytes.Load()
	if n <= tsCacheSeenBytes || n != c1.bytes.Load() {
		t.Fatalf("unexpected total cache size %d; want %d", n, c1.bytes.Load())
	}

	// The limit is shared by all the caches: no markers and no entries above it.
	SetTimestampsCacheMaxBytes(int(n))
	other := jitterTimestamps(2, 4096)
	loadBlock(t, &b, &c2, other)
	loadBlock(t, &b, &c2, other)
	if c2.bytes.Load() != 0 || tsCacheBytes.Load() != n {
		t.Fatalf("cache must reject data above the limit; got c2=%d total=%d", c2.bytes.Load(), tsCacheBytes.Load())
	}

	c1.Reset()
	if tsCacheBytes.Load() != 0 {
		t.Fatalf("Reset must release the size; got %d", tsCacheBytes.Load())
	}
}

func TestTimestampsCache(t *testing.T) {
	SetTimestampsCacheMaxBytes(1 << 20)
	defer SetTimestampsCacheMaxBytes(0)

	var c TimestampsCache
	var b Block
	a, other := jitterTimestamps(1, 4096), jitterTimestamps(2, 4096)
	hits := func() int64 { return int64(tsCacheHits.Get()) }

	// 1st sighting: miss, marker only. 2nd: miss, entry inserted. 3rd: hit.
	h := hits()
	for i := range 3 {
		loadBlock(t, &b, &c, a)
		if !reflect.DeepEqual(b.timestamps, a) {
			t.Fatalf("iteration %d: timestamps differ from fresh decode", i)
		}
		if want := h + int64(i/2); hits() != want {
			t.Fatalf("iteration %d: hits=%d, want %d", i, hits(), want)
		}
	}

	// A hit must not alias cached data: overwrite b and decode again.
	b.timestamps[0] = -1
	loadBlock(t, &b, &c, other)
	loadBlock(t, &b, &c, a)
	if !reflect.DeepEqual(b.timestamps, a) {
		t.Fatalf("cached timestamps differ from fresh decode")
	}

	// Hash collision: same key, other bytes => the entry must be verified and ignored.
	c.m.Range(func(k, v any) bool {
		if v.(*tsCacheEntry) == tsCacheSeen {
			return true
		}
		e := *v.(*tsCacheEntry)
		e.data = append([]byte{}, e.data...)
		e.data[len(e.data)-1] ^= 0xff
		e.timestamps = jitterTimestamps(3, 4096) // what a colliding block would have cached
		c.m.Store(k, &e)
		return true
	})
	loadBlock(t, &b, &c, a)
	if !reflect.DeepEqual(b.timestamps, a) {
		t.Fatalf("collision returned wrong timestamps")
	}

	// A marker never returns data, even if another payload hits the same key.
	c.m.Range(func(k, _ any) bool {
		c.m.Store(k, tsCacheSeen)
		return true
	})
	loadBlock(t, &b, &c, a)
	loadBlock(t, &b, &c, a)
	if !reflect.DeepEqual(b.timestamps, a) {
		t.Fatalf("marker returned wrong timestamps")
	}

	// Concurrent use.
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			var b Block
			for range 20 {
				ts := jitterTimestamps(int64(i%3), 1024)
				loadBlock(t, &b, &c, ts)
				if !reflect.DeepEqual(b.timestamps, ts) {
					t.Errorf("wrong timestamps")
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestTimestampsCacheProbe(t *testing.T) {
	SetTimestampsCacheMaxBytes(1 << 30)
	defer SetTimestampsCacheMaxBytes(0)

	var c TimestampsCache
	var b Block

	// No list repeats: the cache switches itself off after tsCacheProbe lookups and stops storing markers.
	for i := range tsCacheProbe + 50 {
		ts := jitterTimestamps(int64(i), 512)
		loadBlock(t, &b, &c, ts)
		if !reflect.DeepEqual(b.timestamps, ts) {
			t.Fatalf("lookup %d: wrong timestamps", i)
		}
	}
	if !c.off.Load() {
		t.Fatalf("cache must be off after %d lookups without hits", tsCacheProbe)
	}
	if want := int64(tsCacheProbe * tsCacheSeenBytes); c.bytes.Load() != want {
		t.Fatalf("off cache must not grow: got %d bytes, want %d", c.bytes.Load(), want)
	}

	// Reset switches it on again.
	c.Reset()
	if c.off.Load() || c.lookups.Load() != 0 {
		t.Fatalf("Reset must re-arm the cache")
	}

	// A repeating list keeps the cache on, even if everything else is unique.
	rep := jitterTimestamps(1, 512)
	for i := range 3 * tsCacheProbe {
		ts := jitterTimestamps(int64(1000+i), 512)
		if i%8 == 0 {
			ts = rep
		}
		loadBlock(t, &b, &c, ts)
		if !reflect.DeepEqual(b.timestamps, ts) {
			t.Fatalf("lookup %d: wrong timestamps", i)
		}
	}
	if c.off.Load() || c.hits.Load() == 0 {
		t.Fatalf("cache with hits must stay on; hits=%d", c.hits.Load())
	}
}
