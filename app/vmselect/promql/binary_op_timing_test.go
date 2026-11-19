package promql

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func BenchmarkBinaryOpOr(b *testing.B) {
	mustParseMetricsQL := func(s string) *metricsql.BinaryOpExpr {
		e, err := metricsql.Parse(s)
		if err != nil {
			b.Fatalf("cannot parse %q: %s", s, err)
		}
		be, ok := e.(*metricsql.BinaryOpExpr)
		if !ok {
			b.Fatalf("expected BinaryOpExpr from %q: %s", s, err)
		}
		return be
	}
	ts := func(name string) *timeseries {
		if len(name) == 0 {
			// scalar
			return &timeseries{
				Timestamps: []int64{1},
				Values:     []float64{1},
			}
		}
		t := &timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte(name),
				// Real series have labels, which make metric name marshaling noticeably more expensive.
				// The labels are the same for all series, so they don't change how series are grouped.
				Tags: []storage.Tag{
					{Key: []byte("job"), Value: []byte("node-exporter")},
					{Key: []byte("instance"), Value: []byte("10.0.0.1:9100")},
					{Key: []byte("env"), Value: []byte("prod")},
				},
			},
			Timestamps: []int64{1},
			Values:     []float64{1},
		}
		// Sort the labels upfront. Otherwise binaryOpOr sorts them in place,
		// which races between parallel benchmark goroutines sharing the same series.
		sortMetricTags(&t.MetricName)
		return t
	}
	tsWithPoints := func(name string, points int) *timeseries {
		t := ts(name)
		t.Timestamps = make([]int64, points)
		t.Values = make([]float64, points)
		for i := range points {
			t.Timestamps[i] = int64(i)
			t.Values[i] = 1
		}
		return t
	}

	b.Run("tss:1 or tss:1", func(b *testing.B) {
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or b"),
			left:  []*timeseries{ts("a")},
			right: []*timeseries{ts("b")},
		}
		benchmarkBinaryOpOr(b, bfa)
	})
	b.Run("tss:1 or tss:1 points:1000", func(b *testing.B) {
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or b"),
			left:  []*timeseries{tsWithPoints("a", 1000)},
			right: []*timeseries{tsWithPoints("b", 1000)},
		}
		benchmarkBinaryOpOr(b, bfa)
	})
	b.Run("tss:1 or tss:1000", func(b *testing.B) {
		right := make([]*timeseries, 1000)
		for i := range right {
			right[i] = ts(fmt.Sprintf(`b{foo="%d"}`, i))
		}
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or b"),
			left:  []*timeseries{ts(`a{foo="0"}`)},
			right: right,
		}
		benchmarkBinaryOpOr(b, bfa)
	})
	b.Run("tss:1000 or tss:1", func(b *testing.B) {
		left := make([]*timeseries, 1000)
		for i := range left {
			left[i] = ts(fmt.Sprintf(`b{foo="%d"}`, i))
		}
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or b"),
			left:  left,
			right: []*timeseries{ts(`b{foo="0"}`)},
		}
		benchmarkBinaryOpOr(b, bfa)
	})
	b.Run("tss:1000 or tss:1000", func(b *testing.B) {
		left, right := make([]*timeseries, 1000), make([]*timeseries, 1000)
		for i := range left {
			left[i] = ts(fmt.Sprintf(`a{foo="%d"}`, i))
			right[i] = ts(fmt.Sprintf(`b{foo="%d"}`, i))
		}
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or b"),
			left:  left,
			right: right,
		}
		benchmarkBinaryOpOr(b, bfa)
	})
	b.Run("tss:1000 or on() vector(0)", func(b *testing.B) {
		left := make([]*timeseries, 1000)
		for i := range left {
			left[i] = ts(fmt.Sprintf(`a{foo="%d"}`, i))
		}
		bfa := &binaryOpFuncArg{
			be:    mustParseMetricsQL("a or on() vector(0)"),
			left:  left,
			right: []*timeseries{ts(``)},
		}
		benchmarkBinaryOpOr(b, bfa)
	})
}

func benchmarkBinaryOpOr(b *testing.B, bfa *binaryOpFuncArg) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := binaryOpOr(bfa)
			if err != nil {
				b.Fatalf("unexpected error: %s", err)
			}

		}
	})
}
