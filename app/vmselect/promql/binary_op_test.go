package promql

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
	"reflect"
	"testing"
)

func Test_binaryOpOr(t *testing.T) {
	// const
	metricsName1 := storage.MetricName{
		MetricGroup: []byte("ts1"),
		Tags: []storage.Tag{
			{
				Key:   []byte("lb1"),
				Value: []byte("lb1"),
			},
			{
				Key:   []byte("lb2"),
				Value: []byte("lb2"), // different
			},
		},
	}
	metricsName2 := storage.MetricName{
		MetricGroup: []byte("ts1"),
		Tags: []storage.Tag{
			{
				Key:   []byte("lb1"),
				Value: []byte("lb1"),
			},
			{
				Key:   []byte("lb2"),
				Value: []byte("lb3"), // different
			},
		},
	}
	timestamps := []int64{0, 1, 2}

	f := func(input *binaryOpFuncArg, expect []*timeseries) {
		t.Helper()

		result, err := binaryOpOr(input)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		sortSeriesByMetricName(expect)
		if len(expect) != len(result) {
			t.Fatalf("unexpected result length; got %d; want %d", len(result), len(expect))
		}
		for i := range result {
			if !reflect.DeepEqual(expect[i].Timestamps, result[i].Timestamps) {
				t.Fatalf("unexpected result timestamps; got %v; want %v", result[i].Timestamps, expect[i].Timestamps)
			}
			if compareValues(expect[i].Values, result[i].Values) != nil {
				t.Fatalf("unexpected result value; got %v; want %v", result[i].Values, expect[i].Values)
			}
		}
	}

	e := getBinaryOpExpr(`ts1{lb1="lb1",lb2="lb2"} or ts1{lb1="lb1",lb2="lb3"}`)
	leftTss := []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, 1, 1},
			Timestamps: timestamps,
		},
	}
	rightTss := []*timeseries{
		{
			MetricName: metricsName2,
			Values:     []float64{2, 2, 2},
			Timestamps: timestamps,
		},
	}
	expect := append(leftTss, rightTss...)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	leftTss = []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, 1, 1},
			Timestamps: timestamps,
		},
	}
	rightTss = []*timeseries{
		{
			MetricName: metricsName2,
			Values:     []float64{2, 2, 2},
			Timestamps: timestamps,
		},
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, leftTss)

	leftTss = []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, 1, 1},
			Timestamps: timestamps,
		},
	}
	rightTss = []*timeseries{
		{
			MetricName: metricsName2,
			Values:     []float64{2, 2, 2},
			Timestamps: timestamps,
		},
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb2) ts1{lb1="lb1", lb2="lb3"}`)
	expect = append(leftTss, rightTss...)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	leftTss = []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, 1, 1},
			Timestamps: timestamps,
		},
	}
	rightTss = []*timeseries{
		{
			MetricName: metricsName2,
			Values:     []float64{2, 2, 2},
			Timestamps: timestamps,
		},
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	leftTss = []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, nan, 1},
			Timestamps: timestamps,
		},
	}
	rightTss = []*timeseries{
		{
			MetricName: metricsName2,
			Values:     []float64{2, 2, nan},
			Timestamps: timestamps,
		},
	}
	expect = []*timeseries{
		{
			MetricName: metricsName1,
			Values:     []float64{1, nan, 1},
			Timestamps: timestamps,
		},
		{
			MetricName: metricsName2,
			Values:     []float64{nan, 2, nan},
			Timestamps: timestamps,
		},
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)
}

func getBinaryOpExpr(metricsQL string) *metricsql.BinaryOpExpr {
	e, _ := metricsql.Parse(metricsQL)
	return e.(*metricsql.BinaryOpExpr)
}
