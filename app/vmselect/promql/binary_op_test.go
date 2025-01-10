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

	// test case: no grouping
	e := getBinaryOpExpr(`ts1{lb1="lb1",lb2="lb2"} or ts1{lb1="lb1",lb2="lb3"}`)
	leftTss := []*timeseries{
		generateTimeseries(metricsName1, []float64{1, 1, 1}, timestamps),
	}
	rightTss := []*timeseries{
		generateTimeseries(metricsName2, []float64{2, 2, 2}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, append(leftTss, rightTss...))

	// test case: on (lb1)
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{1, 1, 1}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{2, 2, 2}, timestamps),
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, leftTss)

	// test case: on (lb2)
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{1, 1, 1}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{2, 2, 2}, timestamps),
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb2) ts1{lb1="lb1", lb2="lb3"}`)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, append(leftTss, rightTss...))

	// test case: on (lb1) with overlap
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{1, nan, 1}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{2, 2, nan}, timestamps),
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or on (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	expect := []*timeseries{
		generateTimeseries(metricsName1, []float64{1, nan, 1}, timestamps),
		generateTimeseries(metricsName2, []float64{nan, 2, nan}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	// test case: ignoring (lb2), equals to on (lb1)
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{1, nan, 1}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{2, 2, nan}, timestamps),
	}
	e = getBinaryOpExpr(`ts1{lb1="lb1", lb2="lb2"} or ignoring (lb2) ts1{lb1="lb1", lb2="lb3"}`)
	expect = []*timeseries{
		generateTimeseries(metricsName1, []float64{1, nan, 1}, timestamps),
		generateTimeseries(metricsName2, []float64{nan, 2, nan}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	// -----------------------------------------------------------------
	timestamps = []int64{0, 1, 2, 3, 4}

	//load 1m
	//foo{job="bar", a="a"} 0 0 1 1 0
	//foo{job="baz", a="a"} 1 1 1 1 1
	//
	//eval range from 0 to 4m step 1m (foo{job="bar"} == 0) or on (a) (foo{job="baz"})
	//foo{job="bar", a="a"} 0 0 _ _ 0
	//foo{job="baz", a="a"} _ _ 1 1 _
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{1, 1, 1, 1, 1}, timestamps),
	}
	e = getBinaryOpExpr(`(ts1{lb1="lb1", lb2="lb2"} == 0) or on (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	expect = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
		generateTimeseries(metricsName2, []float64{nan, nan, 1, 1, nan}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	//load 1m
	//foo{job="bar", a="a"} 0 0 1 1 0
	//foo{job="baz", a="a"} 1 1 1 1 1
	//
	//eval range from 0 to 4m step 1m (foo{job="bar"} == 2) or on (a) (foo{job="baz"})
	//foo{job="baz", a="a"} 1 1 1 1 1
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{nan, nan, nan, nan, nan}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(metricsName2, []float64{1, 1, 1, 1, 1}, timestamps),
	}
	e = getBinaryOpExpr(`(ts1{lb1="lb1", lb2="lb2"} == 2) or ignoring (lb1) ts1{lb1="lb1", lb2="lb3"}`)
	f(&binaryOpFuncArg{e, leftTss, rightTss}, rightTss)

	//load 1m
	//foo{job="bar", a="a"} 0 0 1 1 0
	//
	//eval range from 0 to 4m step 1m (foo{job="bar"} == 0) or vector(2)
	//foo{job="bar", a="a"} 0 0 _ _ 0
	//{} 2 2 2 2 2
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
	}
	vector2 := storage.MetricName{
		MetricGroup: []byte("vector2"),
		Tags:        []storage.Tag{},
	}
	rightTss = []*timeseries{
		generateTimeseries(vector2, []float64{2, 2, 2, 2, 2}, timestamps),
	}
	e = getBinaryOpExpr(`(ts1{lb1="lb1", lb2="lb2"} == 0) or vector(2)`)
	expect = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
		generateTimeseries(vector2, []float64{2, 2, 2, 2, 2}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

	//load 1m
	//foo{job="bar", a="a"} 0 0 1 1 0
	//
	//eval range from 0 to 4m step 1m (foo{job="bar"} == 0) or vector(2)
	//foo{job="bar", a="a"} 0 0 _ _ 0
	//{} 2 2 2 2 2
	leftTss = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
	}
	rightTss = []*timeseries{
		generateTimeseries(vector2, []float64{2, 2, 2, 2, 2}, timestamps),
	}
	e = getBinaryOpExpr(`(ts1{lb1="lb1", lb2="lb2"} == 0) or on() vector(2)`)
	expect = []*timeseries{
		generateTimeseries(metricsName1, []float64{0, 0, nan, nan, 0}, timestamps),
		generateTimeseries(vector2, []float64{nan, nan, 2, 2, nan}, timestamps),
	}
	f(&binaryOpFuncArg{e, leftTss, rightTss}, expect)

}

func getBinaryOpExpr(metricsQL string) *metricsql.BinaryOpExpr {
	e, _ := metricsql.Parse(metricsQL)
	return e.(*metricsql.BinaryOpExpr)
}

func generateTimeseries(metricsName storage.MetricName, values []float64, t []int64) *timeseries {
	return &timeseries{
		MetricName: metricsName,
		Values:     values,
		Timestamps: t,
	}
}
