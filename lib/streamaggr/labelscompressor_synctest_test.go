//go:build synctest

package streamaggr

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAggregatorsOutputUnchangedByLabelsCompressorCleanup(t *testing.T) {
	const config = `
- match: '{__name__=~".+_total"}'
  without: [node]
  interval: 30s
  outputs: [total_prometheus, increase, rate_sum, count_series]
- match: '{__name__=~".+_seconds"}'
  without: [node]
  interval: 30s
  outputs: [avg, max, last]
`
	run := func(cleanupEvery int) (string, int) {
		var output string
		freed := 0
		synctest.Test(t, func(t *testing.T) {
			resetLabelsCompressor(t)
			var tssOutput []prompb.TimeSeries
			var mu sync.Mutex
			a, err := LoadFromData([]byte(config), func(tss []prompb.TimeSeries) {
				mu.Lock()
				tssOutput = appendClonedTimeseries(tssOutput, tss)
				mu.Unlock()
			}, nil, "cleanup")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			// Push between flushes, so the order is always the same.
			time.Sleep(time.Second)
			for step := range 120 {
				var tss []prompb.TimeSeries
				ts := time.Now().UnixMilli()
				for i := range 5 {
					node := prompb.Label{Name: "node", Value: fmt.Sprintf("n%d", i%2)}
					tss = append(tss,
						prompb.TimeSeries{Labels: []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "svc", Value: fmt.Sprintf("stable-%d", i)}, node},
							Samples: []prompb.Sample{{Value: float64(step * (i + 1)), Timestamp: ts}}},
						prompb.TimeSeries{Labels: []prompb.Label{{Name: "__name__", Value: "latency_seconds"}, {Name: "svc", Value: fmt.Sprintf("stable-%d", i)}, node},
							Samples: []prompb.Sample{{Value: float64(step%7) / 10, Timestamp: ts}}},
					)
				}
				// A churning resource lives for four steps. One comes back after an hour.
				for _, id := range []int{step / 4, (step / 4) % 3} {
					tss = append(tss, prompb.TimeSeries{
						Labels:  []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "resource_id", Value: fmt.Sprintf("r-%d", id)}, {Name: "node", Value: "n0"}},
						Samples: []prompb.Sample{{Value: float64(step), Timestamp: ts}},
					})
				}
				a.Push(tss, nil)
				if cleanupEvery > 0 && step%cleanupEvery == 0 {
					freed += cleanupLabelsCompressor()
				}
				time.Sleep(10 * time.Second)
			}
			a.MustStop()
			output = timeSeriessToString(tssOutput)
		})
		return output, freed
	}

	want, _ := run(0)
	if again, _ := run(0); again != want {
		t.Fatalf("two runs without Cleanup differ; the comparison is not deterministic;\ngot\n%s\nwant\n%s", again, want)
	}
	// Cleanup after every push, much more often than staleness_interval.
	// It must still free only labels that no aggregator state uses.
	got, freed := run(1)
	if freed == 0 {
		t.Fatalf("Cleanup freed no labels; the test did not exercise eviction")
	}
	if got != want {
		t.Fatalf("aggregation output changed by Cleanup;\ngot\n%s\nwant\n%s", got, want)
	}
}

// resetLabelsCompressor gives the test an empty lc and turns off the automatic cleanup,
// so only the test frees labels. Previous tests leave times from other synctest bubbles.
func resetLabelsCompressor(t *testing.T) {
	t.Helper()
	lcUsersLock.Lock()
	n := len(lcUsers)
	lcUsersLock.Unlock()
	if n > 0 {
		t.Fatalf("%d aggregators or deduplicators of previous tests are still running", n)
	}
	lcCleanupWG.Wait()
	lc = promutil.LabelsCompressor{}
	lcNextCleanupMsec.Store(0)
	lcClockMsec.Store(0)
	cleanupInterval := *labelsCompressorCleanupInterval
	*labelsCompressorCleanupInterval = 0
	t.Cleanup(func() {
		*labelsCompressorCleanupInterval = cleanupInterval
	})
}

func TestLabelsCompressorCleanupWaitsForAggregatorFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resetLabelsCompressor(t)
		// An aggregator that has not flushed yet still holds all its keys,
		// so no label can be freed before it flushes.
		a, err := LoadFromData([]byte(`
- match: '{__name__="requests_total"}'
  without: [node]
  interval: 1h
  outputs: [total]
`), func([]prompb.TimeSeries) {}, nil, "slow")
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		time.Sleep(time.Second)
		for i := range 50 {
			a.Push([]prompb.TimeSeries{{
				Labels:  []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "id", Value: fmt.Sprintf("slow-%d", i)}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: time.Now().UnixMilli()}},
			}}, nil)
			time.Sleep(time.Minute)
			if freed := cleanupLabelsCompressor(); freed != 0 {
				t.Fatalf("Cleanup freed %d labels while an aggregator holding them had not flushed", freed)
			}
		}
		a.MustStop()
	})
}

func TestLabelsCompressorCleanupWaitsForPushInFlight(t *testing.T) {
	// A Push that compressed its keys but did not store them yet. A flush doesn't see these keys,
	// so no flush can report them as gone until the Push ends.
	waitsForPush := func(t *testing.T, pushes *pushesInFlight, keysGoneBeforeMsec *atomic.Int64) {
		t.Helper()
		keysGoneBefore := func() time.Time {
			return time.UnixMilli(keysGoneBeforeMsec.Load())
		}
		started, inFlight := pushes.start()
		for range 600 {
			time.Sleep(time.Second)
			if got := keysGoneBefore(); got.After(started) {
				t.Fatalf("keys reported gone before %s while a Push started at %s is in flight", got, started)
			}
		}
		pushes.done(inFlight)
		time.Sleep(time.Minute)
		if got := keysGoneBefore(); !got.After(started) {
			t.Fatalf("keys still reported gone only before %s after the Push completed", got)
		}
	}
	loadAggregator := func(t *testing.T, option string) *aggregator {
		a, err := LoadFromData([]byte(`
- match: '{__name__="requests_total"}'
  interval: 10s
  `+option+`
  outputs: [total]
`), func([]prompb.TimeSeries) {}, nil, "in_flight")
		if err != nil {
			t.Fatalf("cannot initialize aggregators: %s", err)
		}
		t.Cleanup(a.MustStop)
		return a.as[0]
	}
	t.Run("aggregator", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a := loadAggregator(t, "dedup_interval: 0s")
			time.Sleep(time.Second)
			waitsForPush(t, &a.pushes, &a.keysGoneBeforeMsec)
		})
	})
	t.Run("aggregator dedup", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a := loadAggregator(t, "dedup_interval: 5s")
			time.Sleep(time.Second)
			waitsForPush(t, &a.pushes, &a.keysGoneBeforeMsec)
		})
	})
	t.Run("aggregator windows", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			a := loadAggregator(t, "enable_windows: true")
			time.Sleep(time.Second)
			waitsForPush(t, &a.pushes, &a.keysGoneBeforeMsec)
		})
	})
	t.Run("deduplicator", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			d := NewDeduplicator(func([]prompb.TimeSeries) {}, false, 10*time.Second, nil, "in_flight")
			defer d.MustStop()
			time.Sleep(time.Second)
			waitsForPush(t, &d.pushes, &d.keysGoneBeforeMsec)
		})
	})
}

func TestLabelsCompressorCleanupKeepsLabelsOfLiveState(t *testing.T) {
	f := func(config string, sampleAhead time.Duration) {
		t.Helper()
		synctest.Test(t, func(t *testing.T) {
			resetLabelsCompressor(t)
			a, err := LoadFromData([]byte(config), func([]prompb.TimeSeries) {}, nil, "dedup")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			defer a.MustStop()
			// Push after the first flush, so older state doesn't hide the keys.
			time.Sleep(time.Minute + time.Second)
			a.Push([]prompb.TimeSeries{{
				Labels:  []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "id", Value: "gone"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: time.Now().Add(sampleAhead).UnixMilli()}},
			}}, nil)
			freed := 0
			for range 600 {
				time.Sleep(time.Second)
				freed += cleanupLabelsCompressor()
				// Decompress panics if a label of a key in the state was freed.
				a.as[0].aggrOutputs.m.Range(func(k, _ any) bool {
					decompressLabels(nil, k.(string))
					return true
				})
			}
			if freed == 0 {
				t.Fatalf("Cleanup freed no labels; the test did not exercise eviction")
			}
		})
	}

	f(`
- match: '{__name__="requests_total"}'
  interval: 1m
  staleness_interval: 3m
  outputs: [total]
`, 0)
	f(`
- match: '{__name__="requests_total"}'
  interval: 1m
  dedup_interval: 1m
  outputs: [total]
`, 0)
	// A sample of the next window waits in the dedup state for two dedup flushes.
	f(`
- match: '{__name__="requests_total"}'
  interval: 1m
  dedup_interval: 1m
  enable_windows: true
  outputs: [total]
`, time.Minute)
}

func TestLabelsCompressorCleanupKeepsLabelsOfDeduplicatorWindows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		resetLabelsCompressor(t)
		d := NewDeduplicator(func([]prompb.TimeSeries) {}, true, time.Minute, nil, "windows")
		defer d.MustStop()
		time.Sleep(time.Minute + time.Second)
		// A sample of the next window waits for two flushes, which decompress its key.
		d.Push([]prompb.TimeSeries{{
			Labels:  []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "id", Value: "gone"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: time.Now().Add(time.Minute).UnixMilli()}},
		}})
		freed := 0
		for range 600 {
			time.Sleep(time.Second)
			freed += cleanupLabelsCompressor()
		}
		if freed == 0 {
			t.Fatalf("Cleanup freed no labels; the test did not exercise eviction")
		}
	})
}

func TestLabelsCompressorCleanupRunsAfterFlushes(t *testing.T) {
	f := func(cleanupInterval time.Duration, freedExpected bool) {
		t.Helper()
		synctest.Test(t, func(t *testing.T) {
			resetLabelsCompressor(t)
			*labelsCompressorCleanupInterval = cleanupInterval
			a, err := LoadFromData([]byte(`
- match: '{__name__="requests_total"}'
  interval: 10s
  outputs: [total]
`), func([]prompb.TimeSeries) {}, nil, "auto")
			if err != nil {
				t.Fatalf("cannot initialize aggregators: %s", err)
			}
			defer a.MustStop()
			time.Sleep(time.Second)
			for i := range 100 {
				a.Push([]prompb.TimeSeries{{
					Labels:  []prompb.Label{{Name: "__name__", Value: "requests_total"}, {Name: "id", Value: fmt.Sprintf("short-lived-%d", i)}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: time.Now().UnixMilli()}},
				}}, nil)
			}
			time.Sleep(10 * time.Minute)
			if freed := lc.ItemsCount() == 0; freed != freedExpected {
				t.Fatalf("unexpected freeing with -streamAggr.labelsCompressorCleanupInterval=%s; got %d labels left", cleanupInterval, lc.ItemsCount())
			}
		})
	}

	f(time.Minute, true)
	f(0, false)
}

func TestLabelsCompressorNowIgnoresClockStepBack(t *testing.T) {
	var pushes pushesInFlight
	started, inFlight := pushes.start()
	pushes.done(inFlight)
	// The synctest clock starts at 2000-01-01, as if the clock went back after the Push.
	synctest.Test(t, func(t *testing.T) {
		if got := labelsCompressorNow(); got.Before(started.Truncate(time.Millisecond)) {
			t.Fatalf("labels moved to standby now would be stamped %s, before a Push that started at %s", got, started)
		}
	})
}
