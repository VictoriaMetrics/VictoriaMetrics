package storage

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"
)

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
	for i := 0; i < rowsPerInsert; i++ {
		r := &rows[i]
		r.PrecisionBits = defaultPrecisionBits
		r.TSID.MetricID = uint64(rand.Intn(tsidsCount) + 1)
		r.Timestamp = timestamp
		r.Value = value

		timestamp += 10 + rand.Int63n(2)
		value += float64(int(rand.NormFloat64() * 5))
	}
	timestampDelta := timestamp - startTimestamp

	insertsCount := int(1e3)
	rowsCountExpected := insertsCount * len(rows)
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(rowsCountExpected))
	tablePath := "./benchmarkTableAddRows"
	for i := 0; i < b.N; i++ {
		tb, err := openTable(tablePath, -1, nilGetDeletedMetricIDs)
		if err != nil {
			b.Fatalf("cannot open table %q: %s", tablePath, err)
		}

		workCh := make(chan struct{}, insertsCount)
		for j := 0; j < insertsCount; j++ {
			workCh <- struct{}{}
		}
		close(workCh)

		doneCh := make(chan struct{})
		gomaxprocs := runtime.GOMAXPROCS(-1)

		for j := 0; j < gomaxprocs; j++ {
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
					if err := tb.AddRows(rowsCopy); err != nil {
						panic(fmt.Errorf("cannot add rows to table %q: %w", tablePath, err))
					}
				}

				doneCh <- struct{}{}
			}(j)
		}

		for j := 0; j < gomaxprocs; j++ {
			<-doneCh
		}

		tb.MustClose()

		// Open the table from files and verify the rows count on it
		tb, err = openTable(tablePath, -1, nilGetDeletedMetricIDs)
		if err != nil {
			b.Fatalf("cannot open table %q: %s", tablePath, err)
		}
		var m TableMetrics
		tb.UpdateMetrics(&m)
		rowsCount := m.BigRowsCount + m.SmallRowsCount
		if rowsCount != uint64(rowsCountExpected) {
			b.Fatalf("unexpected rows count in the final table %q: got %d; want %d", tablePath, rowsCount, rowsCountExpected)
		}
		tb.MustClose()

		// Remove the table.
		if err := os.RemoveAll(tablePath); err != nil {
			b.Fatalf("cannot remove table %q: %s", tablePath, err)
		}
	}
}
