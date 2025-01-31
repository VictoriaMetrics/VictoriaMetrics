package storage

import (
	"fmt"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func BenchmarkSearch_VariousTimeRanges(b *testing.B) {
	f := func(b *testing.B, tr TimeRange) {
		b.Helper()

		const numRows = 10_000
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
		s.AddRows(want, defaultPrecisionBits)
		s.DebugFlush()

		tfss := NewTagFilters()
		if err := tfss.Add(nil, []byte("metric_.*"), false, true); err != nil {
			b.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}

		// Reset timer to exclude expensive initialization from measurement.
		b.ResetTimer()

		type metricBlock struct {
			MetricName []byte
			Block      *Block
		}
		var mbs []metricBlock
		for range b.N {
			mbs = nil
			var search Search
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
		}

		// Stop timer to exclude expensive correctness check and cleanup from
		// measurement.
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
		if diff := cmp.Diff(mrsToString(want), mrsToString(got)); diff != "" {
			b.Errorf("unexpected metric names (-want, +got):\n%s", diff)
		}

		s.MustClose()
		fs.MustRemoveAll(b.Name())

		// Start timer again to conclude the benchmark correctly.
		b.StartTimer()
	}

	benchmarkStorageOpOnVariousTimeRanges(b, f)
}
