package storage

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	n := m.Run()
	if err := os.RemoveAll("./benchmarkTableSearch"); err != nil {
		panic(fmt.Errorf("cannot remove benchmark tables: %w", err))
	}
	os.Exit(n)
}

func BenchmarkTableSearch(b *testing.B) {
	for _, rowsCount := range []int{1e5, 1e6, 1e7, 1e8} {
		b.Run(fmt.Sprintf("rowsCount_%d", rowsCount), func(b *testing.B) {
			for _, tsidsCount := range []int{1e3, 1e4} {
				b.Run(fmt.Sprintf("tsidsCount_%d", tsidsCount), func(b *testing.B) {
					for _, tsidsSearch := range []int{1, 1e1, 1e2, 1e3, 1e4} {
						b.Run(fmt.Sprintf("tsidsSearch_%d", tsidsSearch), func(b *testing.B) {
							for _, fetchData := range []bool{true, false} {
								b.Run(fmt.Sprintf("fetchData_%v", fetchData), func(b *testing.B) {
									benchmarkTableSearch(b, rowsCount, tsidsCount, tsidsSearch, fetchData)
								})
							}
						})
					}
				})
			}
		})
	}
}

func openBenchTable(b *testing.B, startTimestamp int64, rowsPerInsert, rowsCount, tsidsCount int) *table {
	b.Helper()

	path := fmt.Sprintf("./benchmarkTableSearch/rows%d_tsids%d", rowsCount, tsidsCount)
	if !createdBenchTables[path] {
		createBenchTable(b, path, startTimestamp, rowsPerInsert, rowsCount, tsidsCount)
		createdBenchTables[path] = true
	}
	tb, err := openTable(path, -1, nilGetDeletedMetricIDs)
	if err != nil {
		b.Fatalf("cnanot open table %q: %s", path, err)
	}

	// Verify rows count in the table opened from files.
	insertsCount := uint64((rowsCount + rowsPerInsert - 1) / rowsPerInsert)
	rowsCountExpected := insertsCount * uint64(rowsPerInsert)
	var m TableMetrics
	tb.UpdateMetrics(&m)
	rowsCountActual := m.BigRowsCount + m.SmallRowsCount
	if rowsCountActual != rowsCountExpected {
		b.Fatalf("unexpected rows count in the table %q; got %d; want %d", path, rowsCountActual, rowsCountExpected)
	}

	return tb
}

var createdBenchTables = make(map[string]bool)

func createBenchTable(b *testing.B, path string, startTimestamp int64, rowsPerInsert, rowsCount, tsidsCount int) {
	b.Helper()

	tb, err := openTable(path, -1, nilGetDeletedMetricIDs)
	if err != nil {
		b.Fatalf("cannot open table %q: %s", path, err)
	}

	insertsCount := uint64((rowsCount + rowsPerInsert - 1) / rowsPerInsert)
	timestamp := uint64(startTimestamp)

	var wg sync.WaitGroup
	for k := 0; k < runtime.GOMAXPROCS(-1); k++ {
		wg.Add(1)
		go func() {
			rows := make([]rawRow, rowsPerInsert)
			value := float64(100)
			for int(atomic.AddUint64(&insertsCount, ^uint64(0))) >= 0 {
				for j := 0; j < rowsPerInsert; j++ {
					ts := atomic.AddUint64(&timestamp, uint64(10+rand.Int63n(2)))
					value += float64(int(rand.NormFloat64() * 5))

					r := &rows[j]
					r.PrecisionBits = defaultPrecisionBits
					r.TSID.MetricID = uint64(rand.Intn(tsidsCount) + 1)
					r.Timestamp = int64(ts)
					r.Value = value
				}
				if err := tb.AddRows(rows); err != nil {
					panic(fmt.Errorf("cannot add %d rows: %w", rowsPerInsert, err))
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()

	tb.MustClose()
}

func benchmarkTableSearch(b *testing.B, rowsCount, tsidsCount, tsidsSearch int, fetchData bool) {
	startTimestamp := timestampFromTime(time.Now()) - 365*24*3600*1000
	rowsPerInsert := getMaxRawRowsPerPartition()

	tb := openBenchTable(b, startTimestamp, rowsPerInsert, rowsCount, tsidsCount)
	tr := TimeRange{
		MinTimestamp: startTimestamp,
		MaxTimestamp: (1 << 63) - 1,
	}

	b.ResetTimer()
	b.ReportAllocs()
	rowsPerBench := int64(float64(rowsCount) * float64(tsidsSearch) / float64(tsidsCount))
	if rowsPerBench > int64(rowsCount) {
		rowsPerBench = int64(rowsCount)
	}
	b.SetBytes(rowsPerBench)
	b.RunParallel(func(pb *testing.PB) {
		var ts tableSearch
		tsids := make([]TSID, tsidsSearch)
		var tmpBlock Block
		for pb.Next() {
			for i := range tsids {
				tsids[i].MetricID = 1 + uint64(i)
			}
			ts.Init(tb, tsids, tr)
			for ts.NextBlock() {
				ts.BlockRef.MustReadBlock(&tmpBlock, fetchData)
			}
			ts.MustClose()
		}
	})
	b.StopTimer()

	tb.MustClose()
}
