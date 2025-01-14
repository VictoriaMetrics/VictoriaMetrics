package storage

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func BenchmarkSearch_VariousTimeRanges(b *testing.B) {
	addRowsThenSearch := func(b *testing.B, numRows int, tr TimeRange) {
		b.Helper()

		// Stop timer to exclude data ingestion from measured time.
		b.StopTimer()

		defer fs.MustRemoveAll(b.Name())

		want := make([]MetricRow, numRows)
		step := (tr.MaxTimestamp - tr.MinTimestamp) / int64(numRows)
		mn := MetricName{
			Tags: []Tag{
				{[]byte("job"), []byte("webservice")},
				{[]byte("instance"), []byte("1.2.3.4")},
			},
		}
		for i := range numRows {
			name := fmt.Sprintf("metric_%d", i)
			mn.MetricGroup = []byte(name)
			want[i].MetricNameRaw = mn.marshalRaw(nil)
			want[i].Timestamp = tr.MinTimestamp + int64(i)*step
			want[i].Value = float64(i)
		}
		s := MustOpenStorage(b.Name(), 0, 0, 0)
		defer s.MustClose()
		s.AddRows(want, defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters()
		if err := tfss.Add(nil, []byte("metric_.*"), false, true); err != nil {
			b.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		type metricBlock struct {
			MetricName []byte
			Block      *Block
		}
		var mbs []metricBlock
		var search Search

		// Start timer before performing the search.
		b.StartTimer()

		search.Init(nil, s, []*TagFilters{tfss}, tr, 1e9, noDeadline)
		for search.NextMetricBlock() {
			var (
				block Block
				mb    metricBlock
			)
			search.MetricBlockRef.BlockRef.MustReadBlock(&block)
			mb.MetricName = append(mb.MetricName, search.MetricBlockRef.MetricName...)
			mb.Block = &block
			mbs = append(mbs, mb)
		}
		if err := search.Error(); err != nil {
			b.Fatalf("search error: %v", err)
		}
		search.MustClose()

		// Stop timer again to exclude checking the correctess of the test from
		// the measured time.
		b.StopTimer()

		var got []MetricRow
		for _, mb := range mbs {
			rb := newTestRawBlock(mb.Block, tr)
			if err := mn.Unmarshal(mb.MetricName); err != nil {
				b.Fatalf("cannot unmarshal MetricName %v: %v", string(mb.MetricName), err)
			}
			metricNameRaw := mn.marshalRaw(nil)
			for i, timestamp := range rb.Timestamps {
				mr := MetricRow{
					MetricNameRaw: metricNameRaw,
					Timestamp:     timestamp,
					Value:         rb.Values[i],
				}
				got = append(got, mr)
			}
		}

		sort.Slice(got, func(i, j int) bool {
			return testMetricRowLess(&got[i], &got[j])
		})
		sort.Slice(want, func(i, j int) bool {
			return testMetricRowLess(&want[i], &want[j])
		})
		if !reflect.DeepEqual(got, want) {
			b.Fatalf("unexpected rows found;\ngot\n%s\nwant\n%s", mrsToString(got), mrsToString(want))
		}

		// Start timer to conclude the benchmark interation correctly.
		b.StartTimer()
	}

	for _, numRows := range []int{1000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("%d-rows", numRows), func(b *testing.B) {
			benchmarkStorageOpOnVariousTimeRanges(b, func(b *testing.B, tr TimeRange) {
				b.Helper()
				for range b.N {
					addRowsThenSearch(b, numRows, tr)
				}
			})
		})
	}
}
