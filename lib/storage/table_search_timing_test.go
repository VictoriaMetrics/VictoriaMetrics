package storage

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

func TestMain(m *testing.M) {
	isDebug = true
	n := m.Run()
	if err := os.RemoveAll("benchmarkTableSearch"); err != nil {
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
							benchmarkTableSearch(b, rowsCount, tsidsCount, tsidsSearch)
						})
					}
				})
			}
		})
	}
}

func openBenchTable(b *testing.B, startTimestamp int64, rowsPerInsert, rowsCount, tsidsCount int) (*table, *Storage) {
	b.Helper()

	path := filepath.Join("benchmarkTableSearch", fmt.Sprintf("rows%d_tsids%d", rowsCount, tsidsCount))
	if !createdBenchTables[path] {
		createBenchTable(b, path, startTimestamp, rowsPerInsert, rowsCount, tsidsCount)
		createdBenchTables[path] = true
	}
	strg := newTestStorage()
	tb := mustOpenTable(path, strg)

	// Verify rows count in the table opened from files.
	insertsCount := uint64((rowsCount + rowsPerInsert - 1) / rowsPerInsert)
	rowsCountExpected := insertsCount * uint64(rowsPerInsert)
	var m TableMetrics
	tb.UpdateMetrics(&m)
	if rowsCount := m.TotalRowsCount(); rowsCount != rowsCountExpected {
		b.Fatalf("unexpected rows count in the table %q; got %d; want %d", path, rowsCount, rowsCountExpected)
	}

	return tb, strg
}

var createdBenchTables = make(map[string]bool)

func createBenchTable(b *testing.B, path string, startTimestamp int64, rowsPerInsert, rowsCount, tsidsCount int) {
	b.Helper()

	strg := newTestStorage()
	tb := mustOpenTable(path, strg)

	var insertsCount atomic.Int64
	insertsCount.Store(int64((rowsCount + rowsPerInsert - 1) / rowsPerInsert))

	var timestamp atomic.Uint64
	timestamp.Store(uint64(startTimestamp))

	var wg sync.WaitGroup
	for k := 0; k < cgroup.AvailableCPUs(); k++ {
		wg.Add(1)
		go func(n int) {
			rng := rand.New(rand.NewSource(int64(n)))
			rows := make([]rawRow, rowsPerInsert)
			value := float64(100)
			for insertsCount.Add(-1) >= 0 {
				for j := 0; j < rowsPerInsert; j++ {
					ts := timestamp.Add(uint64(10 + rng.Int63n(2)))
					value += float64(int(rng.NormFloat64() * 5))

					r := &rows[j]
					r.PrecisionBits = defaultPrecisionBits
					r.TSID.MetricID = uint64(rng.Intn(tsidsCount) + 1)
					r.Timestamp = int64(ts)
					r.Value = value
				}
				tb.MustAddRows(rows)
			}
			wg.Done()
		}(k)
	}
	wg.Wait()

	tb.MustClose()
	stopTestStorage(strg)
}

func benchmarkTableSearch(b *testing.B, rowsCount, tsidsCount, tsidsSearch int) {
	startTimestamp := timestampFromTime(time.Now()) - 365*24*3600*1000
	rowsPerInsert := maxRawRowsPerShard

	tb, strg := openBenchTable(b, startTimestamp, rowsPerInsert, rowsCount, tsidsCount)
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
				ts.BlockRef.MustReadBlock(&tmpBlock)
			}
			ts.MustClose()
		}
	})
	b.StopTimer()

	tb.MustClose()
	stopTestStorage(strg)
}
