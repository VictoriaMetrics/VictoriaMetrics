package promql

import (
	"math"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

var (
	testValues     = []float64{123, 34, 44, 21, 54, 34, 99, 12, 44, 32, 34, 34}
	testTimestamps = []int64{5, 15, 24, 36, 49, 60, 78, 80, 97, 115, 120, 130}
)

func TestRollupOutlierIQR(t *testing.T) {
	f := func(values []float64, resultExpected float64) {
		t.Helper()
		rfa := &rollupFuncArg{
			values:     values,
			timestamps: nil,
		}
		result := rollupOutlierIQR(rfa)
		if math.IsNaN(result) {
			if !math.IsNaN(resultExpected) {
				t.Fatalf("unexpected value; got %v; want %v", result, resultExpected)
			}
		} else {
			if math.IsNaN(resultExpected) {
				t.Fatalf("unexpected value; got %v; want %v", result, resultExpected)
			}
			if result != resultExpected {
				t.Fatalf("unexpected value; got %v; want %v", result, resultExpected)
			}
		}
	}

	f([]float64{1, 2, 3, 4, 5}, nan)
	f([]float64{1, 2, 3, 4, 7}, nan)
	f([]float64{1, 2, 3, 4, 8}, 8)
	f([]float64{1, 2, 3, 4, -2}, nan)
	f([]float64{1, 2, 3, 4, -3}, -3)
}

func TestRollupIderivDuplicateTimestamps(t *testing.T) {
	rfa := &rollupFuncArg{
		values:     []float64{1, 2, 3, 4, 5},
		timestamps: []int64{100, 100, 200, 300, 300},
	}
	n := rollupIderiv(rfa)
	if n != 20 {
		t.Fatalf("unexpected value; got %v; want %v", n, 20)
	}

	rfa = &rollupFuncArg{
		values:     []float64{1, 2, 3, 4, 5},
		timestamps: []int64{100, 100, 300, 300, 300},
	}
	n = rollupIderiv(rfa)
	if n != 15 {
		t.Fatalf("unexpected value; got %v; want %v", n, 15)
	}

	rfa = &rollupFuncArg{
		prevValue:  nan,
		values:     []float64{},
		timestamps: []int64{},
	}
	n = rollupIderiv(rfa)
	if !math.IsNaN(n) {
		t.Fatalf("unexpected value; got %v; want %v", n, nan)
	}

	rfa = &rollupFuncArg{
		prevValue:  nan,
		values:     []float64{15},
		timestamps: []int64{100},
	}
	n = rollupIderiv(rfa)
	if !math.IsNaN(n) {
		t.Fatalf("unexpected value; got %v; want %v", n, nan)
	}

	rfa = &rollupFuncArg{
		prevTimestamp: 90,
		prevValue:     10,
		values:        []float64{15},
		timestamps:    []int64{100},
	}
	n = rollupIderiv(rfa)
	if n != 500 {
		t.Fatalf("unexpected value; got %v; want %v", n, 500)
	}

	rfa = &rollupFuncArg{
		prevTimestamp: 100,
		prevValue:     10,
		values:        []float64{15},
		timestamps:    []int64{100},
	}
	n = rollupIderiv(rfa)
	if n != inf {
		t.Fatalf("unexpected value; got %v; want %v", n, inf)
	}

	rfa = &rollupFuncArg{
		prevTimestamp: 100,
		prevValue:     10,
		values:        []float64{15, 20},
		timestamps:    []int64{100, 100},
	}
	n = rollupIderiv(rfa)
	if n != inf {
		t.Fatalf("unexpected value; got %v; want %v", n, inf)
	}
}

func TestRemoveCounterResets(t *testing.T) {
	removeCounterResets(nil)

	values := append([]float64{}, testValues...)
	removeCounterResets(values)
	valuesExpected := []float64{123, 157, 167, 188, 221, 255, 320, 332, 364, 396, 398, 398}
	testRowsEqual(t, values, testTimestamps, valuesExpected, testTimestamps)

	// removeCounterResets doesn't expect negative values, so it doesn't work properly with them.
	values = []float64{-100, -200, -300, -400}
	removeCounterResets(values)
	valuesExpected = []float64{-100, -100, -100, -100}
	timestampsExpected := []int64{0, 1, 2, 3}
	testRowsEqual(t, values, timestampsExpected, valuesExpected, timestampsExpected)

	// verify how partial counter reset is handled.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2787
	values = []float64{100, 95, 120, 119, 139, 50}
	removeCounterResets(values)
	valuesExpected = []float64{100, 100, 125, 125, 145, 195}
	timestampsExpected = []int64{0, 1, 2, 3, 4, 5}
	testRowsEqual(t, values, timestampsExpected, valuesExpected, timestampsExpected)

	// verify results always increase monotonically with possible float operations precision error
	values = []float64{34.094223, 2.7518, 2.140669, 0.044878, 1.887095, 2.546569, 2.490149, 0.045, 0.035684, 0.062454, 0.058296}
	removeCounterResets(values)
	var prev float64
	for i, v := range values {
		if v < prev {
			t.Fatalf("error: unexpected value keep getting bigger %d; cur %v; pre %v\n", i, v, prev)
		}
		prev = v
	}
}

func TestDeltaValues(t *testing.T) {
	deltaValues(nil)

	values := []float64{123}
	deltaValues(values)
	valuesExpected := []float64{0}
	testRowsEqual(t, values, testTimestamps[:1], valuesExpected, testTimestamps[:1])

	values = append([]float64{}, testValues...)
	deltaValues(values)
	valuesExpected = []float64{-89, 10, -23, 33, -20, 65, -87, 32, -12, 2, 0, 0}
	testRowsEqual(t, values, testTimestamps, valuesExpected, testTimestamps)

	// remove counter resets
	values = append([]float64{}, testValues...)
	removeCounterResets(values)
	deltaValues(values)
	valuesExpected = []float64{34, 10, 21, 33, 34, 65, 12, 32, 32, 2, 0, 0}
	testRowsEqual(t, values, testTimestamps, valuesExpected, testTimestamps)
}

func TestDerivValues(t *testing.T) {
	derivValues(nil, nil)

	values := []float64{123}
	derivValues(values, testTimestamps[:1])
	valuesExpected := []float64{0}
	testRowsEqual(t, values, testTimestamps[:1], valuesExpected, testTimestamps[:1])

	values = append([]float64{}, testValues...)
	derivValues(values, testTimestamps)
	valuesExpected = []float64{-8900, 1111.111111111111, -1916.6666666666665, 2538.4615384615386, -1818.1818181818182, 3611.1111111111113,
		-43500, 1882.3529411764705, -666.6666666666667, 400, 0, 0}
	testRowsEqual(t, values, testTimestamps, valuesExpected, testTimestamps)

	// remove counter resets
	values = append([]float64{}, testValues...)
	removeCounterResets(values)
	derivValues(values, testTimestamps)
	valuesExpected = []float64{3400, 1111.111111111111, 1750, 2538.4615384615386, 3090.909090909091, 3611.1111111111113,
		6000, 1882.3529411764705, 1777.7777777777778, 400, 0, 0}
	testRowsEqual(t, values, testTimestamps, valuesExpected, testTimestamps)

	// duplicate timestamps
	values = []float64{1, 2, 3, 4, 5, 6, 7}
	timestamps := []int64{100, 100, 200, 200, 300, 400, 400}
	derivValues(values, timestamps)
	valuesExpected = []float64{0, 20, 20, 20, 10, 10, 10}
	testRowsEqual(t, values, timestamps, valuesExpected, timestamps)
}

func testRollupFunc(t *testing.T, funcName string, args []interface{}, vExpected float64) {
	t.Helper()
	nrf := getRollupFunc(funcName)
	if nrf == nil {
		t.Fatalf("cannot obtain %q", funcName)
	}
	rf, err := nrf(args)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	var rfa rollupFuncArg
	rfa.prevValue = nan
	rfa.prevTimestamp = 0
	rfa.values = append(rfa.values, testValues...)
	rfa.timestamps = append(rfa.timestamps, testTimestamps...)
	rfa.window = rfa.timestamps[len(rfa.timestamps)-1] - rfa.timestamps[0]
	if rollupFuncsRemoveCounterResets[funcName] {
		removeCounterResets(rfa.values)
	}
	for i := 0; i < 5; i++ {
		v := rf(&rfa)
		if math.IsNaN(vExpected) {
			if !math.IsNaN(v) {
				t.Fatalf("unexpected value; got %v; want %v", v, vExpected)
			}
		} else {
			if math.IsNaN(v) {
				t.Fatalf("unexpected value; got %v want %v", v, vExpected)
			}
			eps := math.Abs(v - vExpected)
			if eps > 1e-14 {
				t.Fatalf("unexpected value; got %v; want %v", v, vExpected)
			}
		}
	}
}

func TestRollupDurationOverTime(t *testing.T) {
	f := func(maxInterval, dExpected float64) {
		t.Helper()
		maxIntervals := []*timeseries{{
			Values:     []float64{maxInterval},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, maxIntervals}
		testRollupFunc(t, "duration_over_time", args, dExpected)
	}
	f(-123, 0)
	f(0, 0)
	f(0.001, 0)
	f(0.005, 0.007)
	f(0.01, 0.036)
	f(0.02, 0.125)
	f(1, 0.125)
	f(100, 0.125)
}

func TestRollupShareLEOverTime(t *testing.T) {
	f := func(le, vExpected float64) {
		t.Helper()
		les := []*timeseries{{
			Values:     []float64{le},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, les}
		testRollupFunc(t, "share_le_over_time", args, vExpected)
	}

	f(-123, 0)
	f(0, 0)
	f(10, 0)
	f(12, 0.08333333333333333)
	f(30, 0.16666666666666666)
	f(50, 0.75)
	f(100, 0.9166666666666666)
	f(123, 1)
	f(1000, 1)
}

func TestRollupShareGTOverTime(t *testing.T) {
	f := func(gt, vExpected float64) {
		t.Helper()
		gts := []*timeseries{{
			Values:     []float64{gt},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, gts}
		testRollupFunc(t, "share_gt_over_time", args, vExpected)
	}

	f(-123, 1)
	f(0, 1)
	f(10, 1)
	f(12, 0.9166666666666666)
	f(30, 0.8333333333333334)
	f(50, 0.25)
	f(100, 0.08333333333333333)
	f(123, 0)
	f(1000, 0)
}

func TestRollupShareEQOverTime(t *testing.T) {
	f := func(eq, vExpected float64) {
		t.Helper()
		eqs := []*timeseries{{
			Values:     []float64{eq},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, eqs}
		testRollupFunc(t, "share_eq_over_time", args, vExpected)
	}

	f(-123, 0)
	f(34, 0.3333333333333333)
	f(44, 0.16666666666666666)
	f(123, 0.08333333333333333)
	f(1000, 0)
}

func TestRollupCountLEOverTime(t *testing.T) {
	f := func(le, vExpected float64) {
		t.Helper()
		les := []*timeseries{{
			Values:     []float64{le},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, les}
		testRollupFunc(t, "count_le_over_time", args, vExpected)
	}

	f(-123, 0)
	f(0, 0)
	f(10, 0)
	f(12, 1)
	f(30, 2)
	f(50, 9)
	f(100, 11)
	f(123, 12)
	f(1000, 12)
}

func TestRollupCountGTOverTime(t *testing.T) {
	f := func(gt, vExpected float64) {
		t.Helper()
		gts := []*timeseries{{
			Values:     []float64{gt},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, gts}
		testRollupFunc(t, "count_gt_over_time", args, vExpected)
	}

	f(-123, 12)
	f(0, 12)
	f(10, 12)
	f(12, 11)
	f(30, 10)
	f(50, 3)
	f(100, 1)
	f(123, 0)
	f(1000, 0)
}

func TestRollupCountEQOverTime(t *testing.T) {
	f := func(eq, vExpected float64) {
		t.Helper()
		eqs := []*timeseries{{
			Values:     []float64{eq},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, eqs}
		testRollupFunc(t, "count_eq_over_time", args, vExpected)
	}

	f(-123, 0)
	f(0, 0)
	f(34, 4)
	f(123, 1)
	f(12, 1)
}

func TestRollupCountNEOverTime(t *testing.T) {
	f := func(ne, vExpected float64) {
		t.Helper()
		nes := []*timeseries{{
			Values:     []float64{ne},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, nes}
		testRollupFunc(t, "count_ne_over_time", args, vExpected)
	}

	f(-123, 12)
	f(0, 12)
	f(34, 8)
	f(123, 11)
	f(12, 11)
}

func TestRollupSumLEOverTime(t *testing.T) {
	f := func(le, vExpected float64) {
		t.Helper()
		les := []*timeseries{{
			Values:     []float64{le},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, les}
		testRollupFunc(t, "sum_le_over_time", args, vExpected)
	}

	f(-123, 0)
	f(0, 0)
	f(10, 0)
	f(12, 12)
	f(30, 33)
	f(50, 289)
	f(100, 442)
	f(123, 565)
	f(1000, 565)
}

func TestRollupSumGTOverTime(t *testing.T) {
	f := func(le, vExpected float64) {
		t.Helper()
		les := []*timeseries{{
			Values:     []float64{le},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, les}
		testRollupFunc(t, "sum_gt_over_time", args, vExpected)
	}

	f(-123, 565)
	f(0, 565)
	f(10, 565)
	f(12, 553)
	f(30, 532)
	f(50, 276)
	f(100, 123)
	f(123, 0)
	f(1000, 0)
}

func TestRollupSumEQOverTime(t *testing.T) {
	f := func(le, vExpected float64) {
		t.Helper()
		les := []*timeseries{{
			Values:     []float64{le},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, les}
		testRollupFunc(t, "sum_eq_over_time", args, vExpected)
	}

	f(-123, 0)
	f(0, 0)
	f(10, 0)
	f(12, 12)
	f(30, 0)
	f(50, 0)
	f(100, 0)
	f(123, 123)
	f(1000, 0)
}

func TestRollupQuantileOverTime(t *testing.T) {
	f := func(phi, vExpected float64) {
		t.Helper()
		phis := []*timeseries{{
			Values:     []float64{phi},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{phis, &metricsql.RollupExpr{Expr: &me}}
		testRollupFunc(t, "quantile_over_time", args, vExpected)
	}

	f(-123, math.Inf(-1))
	f(-0.5, math.Inf(-1))
	f(0, 12)
	f(0.1, 22.1)
	f(0.5, 34)
	f(0.9, 94.50000000000001)
	f(1, 123)
	f(234, math.Inf(+1))
}

func TestRollupPredictLinear(t *testing.T) {
	f := func(sec, vExpected float64) {
		t.Helper()
		secs := []*timeseries{{
			Values:     []float64{sec},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, secs}
		testRollupFunc(t, "predict_linear", args, vExpected)
	}

	f(0e-3, 65.07405077267295)
	f(50e-3, 51.7311206569699)
	f(100e-3, 38.38819054126685)
	f(200e-3, 11.702330309860756)
}

func TestLinearRegression(t *testing.T) {
	f := func(values []float64, timestamps []int64, expV, expK float64) {
		t.Helper()
		v, k := linearRegression(values, timestamps, timestamps[0]+100)
		if err := compareValues([]float64{v}, []float64{expV}); err != nil {
			t.Fatalf("unexpected v err: %s", err)
		}
		if err := compareValues([]float64{k}, []float64{expK}); err != nil {
			t.Fatalf("unexpected k err: %s", err)
		}
	}

	f([]float64{1}, []int64{1}, math.NaN(), math.NaN())
	f([]float64{1, 2}, []int64{100, 300}, 1.5, 5)
	f([]float64{2, 4, 6, 8, 10}, []int64{100, 200, 300, 400, 500}, 4, 20)
}

func TestRollupHoltWinters(t *testing.T) {
	f := func(sf, tf, vExpected float64) {
		t.Helper()
		sfs := []*timeseries{{
			Values:     []float64{sf},
			Timestamps: []int64{123},
		}}
		tfs := []*timeseries{{
			Values:     []float64{tf},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}, sfs, tfs}
		testRollupFunc(t, "holt_winters", args, vExpected)
	}

	f(-1, 0.5, nan)
	f(0, 0.5, -856)
	f(1, 0.5, 34)
	f(2, 0.5, nan)
	f(0.5, -1, nan)
	f(0.5, 0, -54.1474609375)
	f(0.5, 1, 25.25)
	f(0.5, 2, nan)
	f(0.5, 0.5, 34.97794532775879)
	f(0.1, 0.5, -131.30529492371622)
	f(0.1, 0.1, -397.3307790780296)
	f(0.5, 0.1, -5.791530520284198)
	f(0.5, 0.9, 25.498906408926757)
	f(0.9, 0.9, 33.99637566941818)
}

func TestRollupHoeffdingBoundLower(t *testing.T) {
	f := func(phi, vExpected float64) {
		t.Helper()
		phis := []*timeseries{{
			Values:     []float64{phi},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{phis, &metricsql.RollupExpr{Expr: &me}}
		testRollupFunc(t, "hoeffding_bound_lower", args, vExpected)
	}

	f(0.5, 28.21949401521037)
	f(-1, 47.083333333333336)
	f(0, 47.083333333333336)
	f(1, -inf)
	f(2, -inf)
	f(0.1, 39.72878000047643)
	f(0.9, 12.701803086472331)
}

func TestRollupHoeffdingBoundUpper(t *testing.T) {
	f := func(phi, vExpected float64) {
		t.Helper()
		phis := []*timeseries{{
			Values:     []float64{phi},
			Timestamps: []int64{123},
		}}
		var me metricsql.MetricExpr
		args := []interface{}{phis, &metricsql.RollupExpr{Expr: &me}}
		testRollupFunc(t, "hoeffding_bound_upper", args, vExpected)
	}

	f(0.5, 65.9471726514563)
	f(-1, 47.083333333333336)
	f(0, 47.083333333333336)
	f(1, inf)
	f(2, inf)
	f(0.1, 54.43788666619024)
	f(0.9, 81.46486358019433)
}

func TestRollupNewRollupFuncSuccess(t *testing.T) {
	f := func(funcName string, vExpected float64) {
		t.Helper()
		var me metricsql.MetricExpr
		args := []interface{}{&metricsql.RollupExpr{Expr: &me}}
		testRollupFunc(t, funcName, args, vExpected)
	}

	f("default_rollup", 34)
	f("changes", 11)
	f("changes_prometheus", 10)
	f("delta", 34)
	f("delta_prometheus", -89)
	f("deriv", -266.85860231406093)
	f("deriv_fast", -712)
	f("idelta", 0)
	f("increase", 398)
	f("increase_prometheus", 275)
	f("irate", 0)
	f("outlier_iqr_over_time", nan)
	f("rate", 2200)
	f("resets", 5)
	f("range_over_time", 111)
	f("avg_over_time", 47.083333333333336)
	f("mad_over_time", 10)
	f("min_over_time", 12)
	f("max_over_time", 123)
	f("tmin_over_time", 0.08)
	f("tmax_over_time", 0.005)
	f("tfirst_over_time", 0.005)
	f("tlast_change_over_time", 0.12)
	f("tlast_over_time", 0.13)
	f("sum_over_time", 565)
	f("sum2_over_time", 37951)
	f("geomean_over_time", 39.33466603189148)
	f("count_over_time", 12)
	f("stale_samples_over_time", 0)
	f("stddev_over_time", 30.752935722554287)
	f("stdvar_over_time", 945.7430555555555)
	f("first_over_time", 123)
	f("last_over_time", 34)
	f("integrate", 0.817)
	f("distinct_over_time", 8)
	f("ideriv", 0)
	f("decreases_over_time", 5)
	f("increases_over_time", 5)
	f("increase_pure", 398)
	f("ascent_over_time", 142)
	f("descent_over_time", 231)
	f("zscore_over_time", -0.4254336383156416)
	f("timestamp", 0.13)
	f("timestamp_with_name", 0.13)
	f("mode_over_time", 34)
	f("rate_over_sum", 4520)
}

func TestRollupNewRollupFuncError(t *testing.T) {
	if nrf := getRollupFunc("non-existing-func"); nrf != nil {
		t.Fatalf("expecting nil func; got %p", nrf)
	}

	f := func(funcName string, args []interface{}) {
		t.Helper()

		nrf := getRollupFunc(funcName)
		rf, err := nrf(args)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if rf != nil {
			t.Fatalf("expecting nil rf; got %p", rf)
		}
	}

	// Invalid number of args
	f("default_rollup", nil)
	f("holt_winters", nil)
	f("predict_linear", nil)
	f("quantile_over_time", nil)
	f("quantiles_over_time", nil)

	// Invalid arg type
	scalarTs := []*timeseries{{
		Values:     []float64{321},
		Timestamps: []int64{123},
	}}
	me := &metricsql.MetricExpr{}
	f("holt_winters", []interface{}{123, 123, 321})
	f("holt_winters", []interface{}{me, 123, 321})
	f("holt_winters", []interface{}{me, scalarTs, 321})
	f("predict_linear", []interface{}{123, 123})
	f("predict_linear", []interface{}{me, 123})
	f("quantile_over_time", []interface{}{123, 123})
	f("quantiles_over_time", []interface{}{123, 123})
}

func TestRollupNoWindowNoPoints(t *testing.T) {
	t.Run("beforeStart", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              0,
			End:                4,
			Step:               1,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 12 {
			t.Fatalf("expecting 12 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, nan, nan, nan, nan}
		timestampsExpected := []int64{0, 1, 2, 3, 4}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("afterEnd", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDelta,
			Start:              120,
			End:                148,
			Step:               4,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned == 0 {
			t.Fatalf("expecting non-zero samplesScanned from rollupConfig.Do")
		}
		valuesExpected := []float64{2, 0, 0, 0, nan, nan, nan, nan}
		timestampsExpected := []int64{120, 124, 128, 132, 136, 140, 144, 148}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupWindowNoPoints(t *testing.T) {
	t.Run("beforeStart", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              0,
			End:                4,
			Step:               1,
			Window:             3,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 12 {
			t.Fatalf("expecting 12 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, nan, nan, nan, nan}
		timestampsExpected := []int64{0, 1, 2, 3, 4}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("afterEnd", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              161,
			End:                191,
			Step:               10,
			Window:             3,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 12 {
			t.Fatalf("expecting 12 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, nan, nan, nan}
		timestampsExpected := []int64{161, 171, 181, 191}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupNoWindowPartialPoints(t *testing.T) {
	t.Run("beforeStart", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              0,
			End:                25,
			Step:               5,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 15 {
			t.Fatalf("expecting 15 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 123, nan, 34, nan, 44}
		timestampsExpected := []int64{0, 5, 10, 15, 20, 25}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("afterEnd", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              100,
			End:                160,
			Step:               20,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 16 {
			t.Fatalf("expecting 16 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{44, 32, 34, nan}
		timestampsExpected := []int64{100, 120, 140, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("middle", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              -50,
			End:                150,
			Step:               50,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, nan, 123, 34, 32}
		timestampsExpected := []int64{-50, 0, 50, 100, 150}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupWindowPartialPoints(t *testing.T) {
	t.Run("beforeStart", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLast,
			Start:              0,
			End:                20,
			Step:               5,
			Window:             8,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 16 {
			t.Fatalf("expecting 16 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 123, 123, 34, 34}
		timestampsExpected := []int64{0, 5, 10, 15, 20}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("afterEnd", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLast,
			Start:              100,
			End:                160,
			Step:               20,
			Window:             18,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 16 {
			t.Fatalf("expecting 16 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{44, 34, 34, nan}
		timestampsExpected := []int64{100, 120, 140, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("middle", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLast,
			Start:              0,
			End:                150,
			Step:               50,
			Window:             19,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 15 {
			t.Fatalf("expecting 15 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 54, 44, nan}
		timestampsExpected := []int64{0, 50, 100, 150}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupFuncsLookbackDelta(t *testing.T) {
	t.Run("1", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              80,
			End:                140,
			Step:               10,
			LookbackDelta:      1,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 18 {
			t.Fatalf("expecting 18 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{99, nan, 44, nan, 32, 34, nan}
		timestampsExpected := []int64{80, 90, 100, 110, 120, 130, 140}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("7", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              80,
			End:                140,
			Step:               10,
			LookbackDelta:      7,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 18 {
			t.Fatalf("expecting 18 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{99, nan, 44, nan, 32, 34, nan}
		timestampsExpected := []int64{80, 90, 100, 110, 120, 130, 140}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("0", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              80,
			End:                140,
			Step:               10,
			LookbackDelta:      0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 18 {
			t.Fatalf("expecting 18 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{99, nan, 44, nan, 32, 34, nan}
		timestampsExpected := []int64{80, 90, 100, 110, 120, 130, 140}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupFuncsNoWindow(t *testing.T) {
	t.Run("first", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupFirst,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 123, 54, 44, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("count", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupCount,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 4, 4, 3, 1}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("min", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupMin,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 21, 12, 32, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("max", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupMax,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 123, 99, 44, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("sum", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupSum,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 222, 199, 110, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("delta", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDelta,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 21, -9, 22, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("delta_prometheus", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDeltaPrometheus,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, -102, -42, -10, nan}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("idelta", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupIdelta,
			Start:              10,
			End:                130,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{123, 33, -87, 0}
		timestampsExpected := []int64{10, 50, 90, 130}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("lag", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLag,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 0.004, 0, 0, 0.03}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("lifetime_1", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLifetime,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 0.031, 0.044, 0.04, 0.01}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("lifetime_2", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupLifetime,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             200,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 47 {
			t.Fatalf("expecting 47 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 0.031, 0.075, 0.115, 0.125}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("scrape_interval_1", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupScrapeInterval,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 0.010333333333333333, 0.011, 0.013333333333333334, 0.01}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("scrape_interval_2", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupScrapeInterval,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             80,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 35 {
			t.Fatalf("expecting 35 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 0.010333333333333333, 0.010714285714285714, 0.012, 0.0125}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("changes", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupChanges,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 4, 4, 3, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("changes_prometheus", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupChangesPrometheus,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 3, 3, 2, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("changes_small_window", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupChanges,
			Start:              0,
			End:                45,
			Step:               9,
			Window:             9,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 16 {
			t.Fatalf("expecting 16 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 1, 1, 1, 1, 0}
		timestampsExpected := []int64{0, 9, 18, 27, 36, 45}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("resets", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupResets,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 2, 2, 1, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("avg", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupAvg,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 55.5, 49.75, 36.666666666666664, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("deriv", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDerivSlow,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, -2879.310344827588, 127.87627310448904, -496.5831435079728, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("deriv_fast", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDerivFast,
			Start:              0,
			End:                20,
			Step:               4,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 14 {
			t.Fatalf("expecting 14 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, nan, nan, 0, -8900, 0}
		timestampsExpected := []int64{0, 4, 8, 12, 16, 20}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("ideriv", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupIderiv,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, -1916.6666666666665, -43500, 400, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("stddev", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupStddev,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 39.81519810323691, 32.080952292598795, 5.2493385826745405, 0}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("integrate", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupIntegrate,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 2.148, 1.593, 1.156, 1.36}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("distinct_over_time_1", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDistinct,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             0,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 24 {
			t.Fatalf("expecting 24 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 4, 4, 3, 1}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("distinct_over_time_2", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupDistinct,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             80,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 35 {
			t.Fatalf("expecting 35 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 4, 7, 6, 3}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("mode_over_time", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupModeOverTime,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             80,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 35 {
			t.Fatalf("expecting 35 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 21, 34, 34, 34}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("rate_over_sum", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupRateOverSum,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             80,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 35 {
			t.Fatalf("expecting 35 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, 2775, 5262.5, 3862.5, 1800}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
	t.Run("zscore_over_time", func(t *testing.T) {
		rc := rollupConfig{
			Func:               rollupZScoreOverTime,
			Start:              0,
			End:                160,
			Step:               40,
			Window:             80,
			MaxPointsPerSeries: 1e4,
		}
		rc.Timestamps = rc.getTimestamps()
		values, samplesScanned := rc.Do(nil, testValues, testTimestamps)
		if samplesScanned != 35 {
			t.Fatalf("expecting 35 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
		}
		valuesExpected := []float64{nan, -0.86650328627136, -1.1200838283548589, -0.40035755084856683, nan}
		timestampsExpected := []int64{0, 40, 80, 120, 160}
		testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
	})
}

func TestRollupBigNumberOfValues(t *testing.T) {
	const srcValuesCount = 1e4
	rc := rollupConfig{
		Func:               rollupDefault,
		End:                srcValuesCount,
		Step:               srcValuesCount / 5,
		Window:             srcValuesCount / 4,
		MaxPointsPerSeries: 1e4,
	}
	rc.Timestamps = rc.getTimestamps()
	srcValues := make([]float64, srcValuesCount)
	srcTimestamps := make([]int64, srcValuesCount)
	for i := 0; i < srcValuesCount; i++ {
		srcValues[i] = float64(i)
		srcTimestamps[i] = int64(i / 2)
	}
	values, samplesScanned := rc.Do(nil, srcValues, srcTimestamps)
	if samplesScanned != 22002 {
		t.Fatalf("expecting 22002 samplesScanned from rollupConfig.Do; got %d", samplesScanned)
	}
	valuesExpected := []float64{1, 4001, 8001, 9999, nan, nan}
	timestampsExpected := []int64{0, 2000, 4000, 6000, 8000, 10000}
	testRowsEqual(t, values, rc.Timestamps, valuesExpected, timestampsExpected)
}

func testRowsEqual(t *testing.T, values []float64, timestamps []int64, valuesExpected []float64, timestampsExpected []int64) {
	t.Helper()
	if len(values) != len(valuesExpected) {
		t.Fatalf("unexpected len(values); got %d; want %d\nvalues=\n%v\nvaluesExpected=\n%v",
			len(values), len(valuesExpected), values, valuesExpected)
	}
	if len(timestamps) != len(timestampsExpected) {
		t.Fatalf("unexpected len(timestamps); got %d; want %d\ntimestamps=\n%v\ntimestampsExpected=\n%v",
			len(timestamps), len(timestampsExpected), timestamps, timestampsExpected)
	}
	if len(values) != len(timestamps) {
		t.Fatalf("len(values) doesn't match len(timestamps); got %d vs %d", len(values), len(timestamps))
	}
	for i, v := range values {
		ts := timestamps[i]
		tsExpected := timestampsExpected[i]
		if ts != tsExpected {
			t.Fatalf("unexpected timestamp at timestamps[%d]; got %d; want %d\ntimestamps=\n%v\ntimestampsExpected=\n%v",
				i, ts, tsExpected, timestamps, timestampsExpected)
		}
		vExpected := valuesExpected[i]
		if math.IsNaN(v) {
			if !math.IsNaN(vExpected) {
				t.Fatalf("unexpected nan value at values[%d]; want %f\nvalues=\n%v\nvaluesExpected=\n%v",
					i, vExpected, values, valuesExpected)
			}
			continue
		}
		if math.IsNaN(vExpected) {
			if !math.IsNaN(v) {
				t.Fatalf("unexpected value at values[%d]; got %f; want nan\nvalues=\n%v\nvaluesExpected=\n%v",
					i, v, values, valuesExpected)
			}
			continue
		}
		// Compare values with the reduced precision because of different precision errors
		// on different OS/architectures. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1738
		// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1653
		if math.Abs(v-vExpected)/math.Abs(vExpected) > 1e-13 {
			t.Fatalf("unexpected value at values[%d]; got %v; want %v\nvalues=\n%v\nvaluesExpected=\n%v",
				i, v, vExpected, values, valuesExpected)
		}
	}
}

func TestRollupDelta(t *testing.T) {
	f := func(prevValue, realPrevValue, realNextValue float64, values []float64, resultExpected float64) {
		t.Helper()
		rfa := &rollupFuncArg{
			prevValue:     prevValue,
			values:        values,
			realPrevValue: realPrevValue,
			realNextValue: realNextValue,
		}
		result := rollupDelta(rfa)
		if math.IsNaN(result) {
			if !math.IsNaN(resultExpected) {
				t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
			}
			return
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}
	f(nan, nan, nan, nil, nan)

	// Small initial value
	f(nan, nan, nan, []float64{1}, 1)
	f(nan, nan, nan, []float64{10}, 0)
	f(nan, nan, nan, []float64{100}, 0)
	f(nan, nan, nan, []float64{1, 2, 3}, 3)
	f(1, nan, nan, []float64{1, 2, 3}, 2)
	f(nan, nan, nan, []float64{5, 6, 8}, 8)
	f(2, nan, nan, []float64{5, 6, 8}, 6)

	f(nan, nan, nan, []float64{100, 100}, 0)

	// Big initial value with zero delta after that.
	f(nan, nan, nan, []float64{1000}, 0)
	f(nan, nan, nan, []float64{1000, 1000}, 0)

	// Big initial value with small delta after that.
	f(nan, nan, nan, []float64{1000, 1001, 1002}, 2)

	// Non-nan realPrevValue
	f(nan, 900, nan, []float64{1000}, 100)
	f(nan, 1000, nan, []float64{1000}, 0)
	f(nan, 1100, nan, []float64{1000}, -100)
	f(nan, 900, nan, []float64{1000, 1001, 1002}, 102)

	// Small delta between realNextValue and values
	f(nan, nan, 990, []float64{1000}, 0)
	f(nan, nan, 1005, []float64{1000}, 0)

	// Big delta between relaNextValue and values
	f(nan, nan, 800, []float64{1000}, 1000)
	f(nan, nan, 1300, []float64{1000}, 1000)

	// Empty values
	f(1, nan, nan, nil, 0)
	f(100, nan, nan, nil, 0)
}
