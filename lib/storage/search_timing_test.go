package storage

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func BenchmarkSearchData_variableSeries(b *testing.B) {
	benchmarkSearch_variableSeries(b, false, benchmarkSearchData)
}

func BenchmarkSearchData_variableDeletedSeries(b *testing.B) {
	benchmarkSearch_variableDeletedSeries(b, false, benchmarkSearchData)
}

func BenchmarkSearchData_variableTimeRange(b *testing.B) {
	benchmarkSearch_variableTimeRange(b, false, benchmarkSearchData)
}

// benchmarkSearchData is a helper function used in various data search
// benchmarks.
func benchmarkSearchData(b *testing.B, s *Storage, tr TimeRange, mrs []MetricRow) {
	b.Helper()
	const (
		accountID = 12
		projectID = 34
	)
	tfss := NewTagFilters(accountID, projectID)
	if err := tfss.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
		b.Fatalf("unexpected error in TagFilters.Add: %v", err)
	}

	type metricBlock struct {
		MetricName []byte
		Block      *Block
	}
	mbs := make([]metricBlock, 0, len(mrs))
	for b.Loop() {
		mbs = mbs[:0]
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

	var mn MetricName
	got := make([]MetricRow, len(mrs))
	for i, mb := range mbs {
		rb := newTestRawBlock(mb.Block, tr)
		if err := mn.Unmarshal(mb.MetricName); err != nil {
			b.Fatalf("cannot unmarshal MetricName %v: %v", string(mb.MetricName), err)
		}
		metricNameRaw := mn.marshalRaw(nil)
		for j, timestamp := range rb.Timestamps {
			mr := MetricRow{
				MetricNameRaw: metricNameRaw,
				Timestamp:     timestamp,
				Value:         rb.Values[j],
			}
			got[i] = mr
		}
	}
	testSortMetricRows(got)
	want := mrs
	testSortMetricRows(want)
	if diff := cmp.Diff(mrsToString(want), mrsToString(got)); diff != "" {
		b.Errorf("unexpected metric rows (-want, +got):\n%s", diff)
	}
}
