package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// BenchmarkTableMustAddRowsBuckets measures routing and synchronous shard writes.
// Preallocated shards are drained after each batch to exclude asynchronous merges.
func BenchmarkTableMustAddRowsBuckets(b *testing.B) {
	b.Logf("rawRow size: %d bytes", unsafe.Sizeof(rawRow{}))
	for _, r := range []int{100, 8000} {
		for _, p := range []int{2, 12} {
			for _, layout := range []string{"grouped", "alternating", "single"} {
				b.Run(fmt.Sprintf("R=%d/P=%d/%s", r, p, layout), func(b *testing.B) {
					tb := newBucketTestTable(p, r)
					rows := make([]rawRow, r)
					for i := range rows {
						idx := 0
						switch layout {
						case "grouped":
							idx = i * p / r
						case "alternating":
							idx = i % p
						}
						rows[i] = rawRow{Timestamp: tb.ptws[idx].pt.tr.MinTimestamp, Value: float64(i), PrecisionBits: defaultPrecisionBits}
					}
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						tb.MustAddRows(rows)
						for _, ptw := range tb.ptws {
							shard := &ptw.pt.rawRows.shards[0]
							shard.rows = shard.rows[:0]
						}
					}
				})
			}
		}
	}
}

func BenchmarkTableAddRows(b *testing.B) {
	for _, tsidsCount := range []int{1e0, 1e1, 1e2, 1e3, 1e4} {
		b.Run(fmt.Sprintf("tsidsCount_%d", tsidsCount), func(b *testing.B) {
			for _, rowsPerInsert := range []int{1, 1e1, 1e2, 1e3, 1e4, 1e5} {
				b.Run(fmt.Sprintf("rowsPerInsert_%d", rowsPerInsert), func(b *testing.B) {
					benchmarkTableAddRows(b, rowsPerInsert, tsidsCount)
				})
			}
		})
	}
}

func benchmarkTableAddRows(b *testing.B, rowsPerInsert, tsidsCount int) {
	rows := make([]rawRow, rowsPerInsert)
	startTimestamp := timestampFromTime(time.Now())
	timestamp := startTimestamp
	value := float64(100)
	rng := rand.New(rand.NewSource(1))
	for i := range rowsPerInsert {
		r := &rows[i]
		r.PrecisionBits = defaultPrecisionBits
		r.TSID.MetricID = uint64(rng.Intn(tsidsCount) + 1)
		r.Timestamp = timestamp
		r.Value = value

		timestamp += 10 + rng.Int63n(2)
		value += float64(int(rng.NormFloat64() * 5))
	}
	timestampDelta := timestamp - startTimestamp

	insertsCount := int(1e3)
	rowsCountExpected := insertsCount * len(rows)
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(rowsCountExpected))
	tablePath := "benchmarkTableAddRows"
	strg := newTestStorage()
	for range b.N {
		tb := mustOpenTable(tablePath, strg)

		workCh := make(chan struct{}, insertsCount)
		for range insertsCount {
			workCh <- struct{}{}
		}
		close(workCh)

		doneCh := make(chan struct{})
		gomaxprocs := cgroup.AvailableCPUs()

		for j := range gomaxprocs {
			go func(goroutineID int) {
				// Make per-goroutine rows copy with distinct timestamps.
				rowsCopy := append([]rawRow{}, rows...)
				for k := range rowsCopy {
					r := &rowsCopy[k]
					r.Timestamp += int64(goroutineID)
					r.Value += float64(goroutineID)
				}

				for range workCh {
					// Update rowsCopy to the next timestamp chunk.
					for q := range rowsCopy {
						r := &rowsCopy[q]
						r.Timestamp += timestampDelta
						r.Value++
					}
					// Add updated rowsCopy.
					tb.MustAddRows(rowsCopy)
				}

				doneCh <- struct{}{}
			}(j)
		}

		for range gomaxprocs {
			<-doneCh
		}

		tb.MustClose()

		// Open the table from files and verify the rows count on it
		tb = mustOpenTable(tablePath, strg)
		var m TableMetrics
		tb.UpdateMetrics(&m)
		if rowsCount := m.TotalRowsCount(); rowsCount != uint64(rowsCountExpected) {
			b.Fatalf("unexpected rows count in the final table %q: got %d; want %d", tablePath, rowsCount, rowsCountExpected)
		}
		tb.MustClose()

		// Remove the table.
		fs.MustRemoveDir(tablePath)
	}
	stopTestStorage(strg)
}
