//go:build synctest

package streamaggr

import (
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

func TestDeduplicatorSlowFlush(t *testing.T) {
	type flushResult struct {
		// flushTime is the time of the day of the flush
		flushTime string
		// sampleTime is the time of the day of the sample timestamp
		sampleTime string
	}

	// f creates a deduplicator at 00:00:00, makes its first non-empty flush take slowFlushDuration,
	// pushes a sample of the same series at every time of the day from pushTimes and stops the deduplicator at stopTime.
	// Then it verifies the flushed samples, the number of skipped flushes and the number of dropped samples.
	// The dropped samples include both the samples removed by deduplication and the samples dropped as too old.
	f := func(enableWindows bool, slowFlushDuration time.Duration, pushTimes []string, stopTime string, resultsExpected []flushResult, flushSkippedExpected, droppedSamplesExpected uint64) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			var results []flushResult
			var resultsLock sync.Mutex
			slowFlushDone := false
			pushFunc := func(tss []prompb.TimeSeries) {
				flushTime := timeOfDay(time.Now().UnixMilli())
				resultsLock.Lock()
				for _, ts := range tss {
					for _, s := range ts.Samples {
						results = append(results, flushResult{
							flushTime:  flushTime,
							sampleTime: timeOfDay(s.Timestamp),
						})
					}
				}
				isSlow := !slowFlushDone
				slowFlushDone = true
				resultsLock.Unlock()
				// Sleep after unlocking, since shards are flushed concurrently,
				// and synctest can't advance the fake clock while other flushes wait for the lock.
				if isSlow {
					time.Sleep(slowFlushDuration)
				}
			}
			d := NewDeduplicator(pushFunc, enableWindows, time.Minute, nil, "some_alias")

			for _, pushTime := range pushTimes {
				time.Sleep(time.Until(at(pushTime)))
				tss := prometheus.MustParsePromMetrics(`foo 1`, time.Now().UnixMilli())
				d.Push(tss)
			}
			time.Sleep(time.Until(at(stopTime)))
			synctest.Wait()

			flushSkipped := d.da.flushSkipped.Get()
			droppedSamples := d.da.droppedSamples.Get()
			d.MustStop()

			if droppedSamples != droppedSamplesExpected {
				t.Fatalf("enableWindows=%v, slowFlushDuration=%s: unexpected number of dropped samples; got %d; want %d", enableWindows, slowFlushDuration, droppedSamples, droppedSamplesExpected)
			}
			if flushSkipped != flushSkippedExpected {
				t.Fatalf("enableWindows=%v, slowFlushDuration=%s: unexpected number of skipped flushes; got %d; want %d", enableWindows, slowFlushDuration, flushSkipped, flushSkippedExpected)
			}
			// shards are flushed concurrently, so sort the results
			slices.SortFunc(results, func(a, b flushResult) int {
				if a.flushTime != b.flushTime {
					return strings.Compare(a.flushTime, b.flushTime)
				}
				return strings.Compare(a.sampleTime, b.sampleTime)
			})
			if !reflect.DeepEqual(results, resultsExpected) {
				t.Fatalf("enableWindows=%v, slowFlushDuration=%s: unexpected results;\ngot\n%v\nwant\n%v", enableWindows, slowFlushDuration, results, resultsExpected)
			}
		})
	}

	s := time.Second

	// The deduplicator doesn't align flushes, so they happen at 00:01:00, 00:02:00, 00:03:00, etc.
	for _, enableWindows := range []bool{false, true} {
		// The first flush at 00:01:00 takes 1.5 intervals, so the flush at 00:02:00 must be skipped.
		// The samples pushed at 00:01:30 and 00:02:36 must be deduplicated together,
		// so only the last one must be flushed at 00:03:00.
		f(enableWindows, 90*s, []string{"00:00:30", "00:01:30", "00:02:36", "00:03:30", "00:04:30"}, "00:05:30", []flushResult{
			{"00:01:00", "00:00:30"},
			{"00:03:00", "00:02:36"},
			{"00:04:00", "00:03:30"},
			{"00:05:00", "00:04:30"},
		}, 1, 1)

		// The first flush at 00:01:00 takes 2.5 intervals, so the flushes at 00:02:00 and 00:03:00 must be skipped.
		// The samples pushed at 00:02:00, 00:03:00 and 00:03:40 must be deduplicated together,
		// so only the last one must be flushed at 00:04:00.
		f(enableWindows, 150*s, []string{"00:00:30", "00:02:00", "00:03:00", "00:03:40", "00:04:30"}, "00:05:30", []flushResult{
			{"00:01:00", "00:00:30"},
			{"00:04:00", "00:03:40"},
			{"00:05:00", "00:04:30"},
		}, 2, 2)
	}
}

func TestDeduplicatorWindowsLateSamples(t *testing.T) {
	type flushResult struct {
		// flushTime is the time of the day of the flush
		flushTime string
		// sampleTime is the time of the day of the sample timestamp
		sampleTime string
	}
	type push struct {
		// pushTime is the time of the day of the push
		pushTime string
		// sampleTime is the time of the day of the sample timestamp
		sampleTime string
	}

	// f creates a deduplicator with enabled windows at 00:00:00, makes every push from pushes with a sample of the same series,
	// and stops the deduplicator at stopTime.
	// Then it verifies the flushed samples and the number of dropped samples.
	// The dropped samples include both the samples removed by deduplication and the samples dropped as too old.
	f := func(pushes []push, stopTime string, resultsExpected []flushResult, droppedSamplesExpected uint64) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			var results []flushResult
			var resultsLock sync.Mutex
			pushFunc := func(tss []prompb.TimeSeries) {
				flushTime := timeOfDay(time.Now().UnixMilli())
				resultsLock.Lock()
				for _, ts := range tss {
					for _, s := range ts.Samples {
						results = append(results, flushResult{
							flushTime:  flushTime,
							sampleTime: timeOfDay(s.Timestamp),
						})
					}
				}
				resultsLock.Unlock()
			}
			d := NewDeduplicator(pushFunc, true, time.Minute, nil, "some_alias")

			for _, p := range pushes {
				time.Sleep(time.Until(at(p.pushTime)))
				tss := prometheus.MustParsePromMetrics(`foo 1`, at(p.sampleTime).UnixMilli())
				d.Push(tss)
			}
			time.Sleep(time.Until(at(stopTime)))
			synctest.Wait()
			droppedSamples := d.da.droppedSamples.Get()
			d.MustStop()

			if droppedSamples != droppedSamplesExpected {
				t.Fatalf("unexpected number of dropped samples; got %d; want %d", droppedSamples, droppedSamplesExpected)
			}
			// shards are flushed concurrently, so sort the results
			slices.SortFunc(results, func(a, b flushResult) int {
				if a.flushTime != b.flushTime {
					return strings.Compare(a.flushTime, b.flushTime)
				}
				return strings.Compare(a.sampleTime, b.sampleTime)
			})
			if !reflect.DeepEqual(results, resultsExpected) {
				t.Fatalf("unexpected results;\ngot\n%v\nwant\n%v", results, resultsExpected)
			}
		})
	}

	// The deduplicator doesn't align flushes, so they happen at 00:01:00, 00:02:00, etc.

	// Samples arrive with a steady 20s lag, so every flush waits 20s for the late samples of the due window.
	// The sample pushed at 00:01:15 belongs to the window ending at 00:01:00,
	// so it must be deduplicated with the sample pushed at 00:00:30 and flushed at 00:01:20.
	// The sample pushed at 00:01:25 arrives after that flush, so it belongs to the next window
	// and must be deduplicated with the sample pushed at 00:02:10.
	f([]push{
		{"00:00:30", "00:00:10"},
		{"00:01:15", "00:00:55"},
		{"00:01:25", "00:01:05"},
		{"00:02:10", "00:01:55"},
	}, "00:02:40", []flushResult{
		{"00:01:20", "00:00:55"},
		{"00:02:20", "00:01:55"},
	}, 2)

	// Samples without lag don't delay the flush at 00:01:00,
	// so the late sample arriving at 00:01:10 after the flush must be dropped as too old.
	// If no samples are accepted since the previous flush, the lag of dropped samples is used for the flush delay,
	// so the late sample arriving at 00:02:10 must be flushed with the window ending at 00:02:00 at 00:02:15.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/pull/11866
	f([]push{
		{"00:00:30", "00:00:30"},
		{"00:01:10", "00:00:55"},
		{"00:02:10", "00:01:55"},
	}, "00:02:40", []flushResult{
		{"00:01:00", "00:00:30"},
		{"00:02:15", "00:01:55"},
	}, 1)

	// The flush delay is capped at 90% of the interval, so with a 59s lag the flush happens at 00:01:54,
	// and a sample arriving at 00:01:56 must be dropped as too old.
	// Stop before the next flush at 00:02:00, since the stop must not depend on the flush delay.
	f([]push{
		{"00:00:59", "00:00:00"},
		{"00:01:56", "00:00:59"},
	}, "00:01:58", []flushResult{
		{"00:01:54", "00:00:00"},
	}, 1)
}
