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
		return &timeseries{
			MetricName: storage.MetricName{
				MetricGroup: []byte(name),
				Tags: []storage.Tag{
					{Key: []byte("job"), Value: []byte("node-exporter")},
					{Key: []byte("instance"), Value: []byte("10.0.0.1:9100")},
					{Key: []byte("env"), Value: []byte("prod")},
				},
			},
			Timestamps: []int64{1},
			Values:     []float64{1},
		}
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
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or b"),
				left:  []*timeseries{ts("a")},
				right: []*timeseries{ts("b")},
			}
		})
	})
	b.Run("tss:1 or tss:1 points:1000", func(b *testing.B) {
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or b"),
				left:  []*timeseries{tsWithPoints("a", 1000)},
				right: []*timeseries{tsWithPoints("b", 1000)},
			}
		})
	})
	b.Run("tss:1 or tss:1000", func(b *testing.B) {
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			right := make([]*timeseries, 1000)
			for i := range right {
				right[i] = ts(fmt.Sprintf("b_%d", i))
			}
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or b"),
				left:  []*timeseries{ts("a_0")},
				right: right,
			}
		})
	})
	b.Run("tss:1000 or tss:1", func(b *testing.B) {
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			left := make([]*timeseries, 1000)
			for i := range left {
				left[i] = ts(fmt.Sprintf("b_%d", i))
			}
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or b"),
				left:  left,
				right: []*timeseries{ts("b_0")},
			}
		})
	})
	b.Run("tss:1000 or tss:1000", func(b *testing.B) {
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			left, right := make([]*timeseries, 1000), make([]*timeseries, 1000)
			for i := range left {
				left[i] = ts(fmt.Sprintf("a_%d", i))
				right[i] = ts(fmt.Sprintf("b_%d", i))
			}
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or b"),
				left:  left,
				right: right,
			}
		})
	})
	b.Run("tss:1000 or on() vector(0)", func(b *testing.B) {
		benchmarkBinaryOpOr(b, func() *binaryOpFuncArg {
			left := make([]*timeseries, 1000)
			for i := range left {
				left[i] = ts(fmt.Sprintf("a_%d", i))
			}
			return &binaryOpFuncArg{
				be:    mustParseMetricsQL("a or on() vector(0)"),
				left:  left,
				right: []*timeseries{ts(``)},
			}
		})
	})
}

func benchmarkBinaryOpOr(b *testing.B, newBfa func() *binaryOpFuncArg) {
	b.RunParallel(func(pb *testing.PB) {
		// binaryOpOr modifies series in place, so every goroutine needs its own series.
		bfa := newBfa()
		for pb.Next() {
			_, err := binaryOpOr(bfa)
			if err != nil {
				b.Fatalf("unexpected error: %s", err)
			}

		}
	})
}
