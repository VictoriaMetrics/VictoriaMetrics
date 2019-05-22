package decimal

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestAppendDecimalToFloat(t *testing.T) {
	testAppendDecimalToFloat(t, []int64{}, 0, nil)
	testAppendDecimalToFloat(t, []int64{0}, 0, []float64{0})
	testAppendDecimalToFloat(t, []int64{0}, 10, []float64{0})
	testAppendDecimalToFloat(t, []int64{0}, -10, []float64{0})
	testAppendDecimalToFloat(t, []int64{-1, -10, 0, 100}, 2, []float64{-1e2, -1e3, 0, 1e4})
	testAppendDecimalToFloat(t, []int64{-1, -10, 0, 100}, -2, []float64{-1e-2, -1e-1, 0, 1})
}

func testAppendDecimalToFloat(t *testing.T, va []int64, e int16, fExpected []float64) {
	f := AppendDecimalToFloat(nil, va, e)
	if !reflect.DeepEqual(f, fExpected) {
		t.Fatalf("unexpected f for va=%d, e=%d: got\n%v; expecting\n%v", va, e, f, fExpected)
	}

	prefix := []float64{1, 2, 3, 4}
	f = AppendDecimalToFloat(prefix, va, e)
	if !reflect.DeepEqual(f[:len(prefix)], prefix) {
		t.Fatalf("unexpected prefix for va=%d, e=%d; got\n%v; expecting\n%v", va, e, f[:len(prefix)], prefix)
	}
	if fExpected == nil {
		fExpected = []float64{}
	}
	if !reflect.DeepEqual(f[len(prefix):], fExpected) {
		t.Fatalf("unexpected prefixed f for va=%d, e=%d: got\n%v; expecting\n%v", va, e, f[len(prefix):], fExpected)
	}
}

func TestCalibrateScale(t *testing.T) {
	testCalibrateScale(t, []int64{}, []int64{}, 0, 0, []int64{}, []int64{}, 0)
	testCalibrateScale(t, []int64{0}, []int64{0}, 0, 0, []int64{0}, []int64{0}, 0)
	testCalibrateScale(t, []int64{0}, []int64{1}, 0, 0, []int64{0}, []int64{1}, 0)
	testCalibrateScale(t, []int64{1, 0, 2}, []int64{5, -3}, 0, 1, []int64{1, 0, 2}, []int64{50, -30}, 0)
	testCalibrateScale(t, []int64{-1, 2}, []int64{5, 6, 3}, 2, -1, []int64{-1000, 2000}, []int64{5, 6, 3}, -1)
	testCalibrateScale(t, []int64{123, -456, 94}, []int64{-9, 4, -3, 45}, -3, -3, []int64{123, -456, 94}, []int64{-9, 4, -3, 45}, -3)
	testCalibrateScale(t, []int64{1e18, 1, 0}, []int64{3, 456}, 0, -2, []int64{1e18, 1, 0}, []int64{0, 4}, 0)
	testCalibrateScale(t, []int64{12345, 678}, []int64{12, -1e17, -3}, -3, 0, []int64{123, 6}, []int64{120, -1e18, -30}, -1)
	testCalibrateScale(t, []int64{1, 2}, nil, 12, 34, []int64{1, 2}, nil, 12)
	testCalibrateScale(t, nil, []int64{3, 1}, 12, 34, nil, []int64{3, 1}, 34)
	testCalibrateScale(t, []int64{923}, []int64{2, 3}, 100, -100, []int64{923e15}, []int64{0, 0}, 85)
	testCalibrateScale(t, []int64{923}, []int64{2, 3}, -100, 100, []int64{0}, []int64{2e18, 3e18}, 82)
	testCalibrateScale(t, []int64{123, 456, 789, 135}, []int64{}, -12, -10, []int64{123, 456, 789, 135}, []int64{}, -12)
	testCalibrateScale(t, []int64{123, 456, 789, 135}, []int64{}, -10, -12, []int64{123, 456, 789, 135}, []int64{}, -10)

	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{500, 100}, 0, 0, []int64{vInfPos, 1200}, []int64{500, 100}, 0)
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{500, 100}, 0, 2, []int64{vInfPos, 1200}, []int64{500e2, 100e2}, 0)
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{500, 100}, 0, -2, []int64{vInfPos, 1200}, []int64{5, 1}, 0)
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{3500, 100}, 0, -3, []int64{vInfPos, 1200}, []int64{3, 0}, 0)
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{35, 1}, 0, 40, []int64{vInfPos, 0}, []int64{35e17, 1e17}, 23)
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{35, 1}, 40, 0, []int64{vInfPos, 1200}, []int64{0, 0}, 40)
	testCalibrateScale(t, []int64{vInfNeg, 1200}, []int64{35, 1}, 35, -5, []int64{vInfNeg, 1200}, []int64{0, 0}, 35)
	testCalibrateScale(t, []int64{vMax, vMin, 123}, []int64{100}, 0, 3, []int64{vMax, vMin, 123}, []int64{100e3}, 0)
	testCalibrateScale(t, []int64{vMax, vMin, 123}, []int64{100}, 3, 0, []int64{vMax, vMin, 123}, []int64{0}, 3)
	testCalibrateScale(t, []int64{vMax, vMin, 123}, []int64{100}, 0, 30, []int64{92233, -92233, 0}, []int64{100e16}, 14)
}

func testCalibrateScale(t *testing.T, a, b []int64, ae, be int16, aExpected, bExpected []int64, eExpected int16) {
	t.Helper()

	if a == nil {
		a = []int64{}
	}
	if b == nil {
		b = []int64{}
	}
	if aExpected == nil {
		aExpected = []int64{}
	}
	if bExpected == nil {
		bExpected = []int64{}
	}

	aCopy := append([]int64{}, a...)
	bCopy := append([]int64{}, b...)
	e := CalibrateScale(aCopy, ae, bCopy, be)
	if e != eExpected {
		t.Fatalf("unexpected e for a=%d, b=%d, ae=%d, be=%d; got %d; expecting %d", a, b, ae, be, e, eExpected)
	}
	if !reflect.DeepEqual(aCopy, aExpected) {
		t.Fatalf("unexpected a for b=%d, ae=%d, be=%d; got\n%d; expecting\n%d", b, ae, be, aCopy, aExpected)
	}
	if !reflect.DeepEqual(bCopy, bExpected) {
		t.Fatalf("unexpected b for a=%d, ae=%d, be=%d; got\n%d; expecting\n%d", a, ae, be, bCopy, bExpected)
	}

	// Try reverse args.
	aCopy = append([]int64{}, a...)
	bCopy = append([]int64{}, b...)
	e = CalibrateScale(bCopy, be, aCopy, ae)
	if e != eExpected {
		t.Fatalf("revers: unexpected e for a=%d, b=%d, ae=%d, be=%d; got %d; expecting %d", a, b, ae, be, e, eExpected)
	}
	if !reflect.DeepEqual(aCopy, aExpected) {
		t.Fatalf("reverse: unexpected a for b=%d, ae=%d, be=%d; got\n%d; expecting\n%d", b, ae, be, aCopy, aExpected)
	}
	if !reflect.DeepEqual(bCopy, bExpected) {
		t.Fatalf("reverse: unexpected b for a=%d, ae=%d, be=%d; got\n%d; expecting\n%d", a, ae, be, bCopy, bExpected)
	}
}

func TestMaxUpExponent(t *testing.T) {
	testMaxUpExponent(t, 0, 1024)
	testMaxUpExponent(t, -1<<63, 0)
	testMaxUpExponent(t, (-1<<63)+1, 0)
	testMaxUpExponent(t, (1<<63)-1, 0)
	testMaxUpExponent(t, 1, 18)
	testMaxUpExponent(t, 12, 17)
	testMaxUpExponent(t, 123, 16)
	testMaxUpExponent(t, 1234, 15)
	testMaxUpExponent(t, 12345, 14)
	testMaxUpExponent(t, 123456, 13)
	testMaxUpExponent(t, 1234567, 12)
	testMaxUpExponent(t, 12345678, 11)
	testMaxUpExponent(t, 123456789, 10)
	testMaxUpExponent(t, 1234567890, 9)
	testMaxUpExponent(t, 12345678901, 8)
	testMaxUpExponent(t, 123456789012, 7)
	testMaxUpExponent(t, 1234567890123, 6)
	testMaxUpExponent(t, 12345678901234, 5)
	testMaxUpExponent(t, 123456789012345, 4)
	testMaxUpExponent(t, 1234567890123456, 3)
	testMaxUpExponent(t, 12345678901234567, 2)
	testMaxUpExponent(t, 123456789012345678, 1)
	testMaxUpExponent(t, 1234567890123456789, 0)
	testMaxUpExponent(t, 923456789012345678, 0)
	testMaxUpExponent(t, 92345678901234567, 1)
	testMaxUpExponent(t, 9234567890123456, 2)
	testMaxUpExponent(t, 923456789012345, 3)
	testMaxUpExponent(t, 92345678901234, 4)
	testMaxUpExponent(t, 9234567890123, 5)
	testMaxUpExponent(t, 923456789012, 6)
	testMaxUpExponent(t, 92345678901, 7)
	testMaxUpExponent(t, 9234567890, 8)
	testMaxUpExponent(t, 923456789, 9)
	testMaxUpExponent(t, 92345678, 10)
	testMaxUpExponent(t, 9234567, 11)
	testMaxUpExponent(t, 923456, 12)
	testMaxUpExponent(t, 92345, 13)
	testMaxUpExponent(t, 9234, 14)
	testMaxUpExponent(t, 923, 15)
	testMaxUpExponent(t, 92, 17)
	testMaxUpExponent(t, 9, 18)
}

func testMaxUpExponent(t *testing.T, v int64, eExpected int16) {
	t.Helper()

	e := maxUpExponent(v)
	if e != eExpected {
		t.Fatalf("unexpected e for v=%d; got %d; epxecting %d", v, e, eExpected)
	}
	e = maxUpExponent(-v)
	if e != eExpected {
		t.Fatalf("unexpected e for v=%d; got %d; expecting %d", -v, e, eExpected)
	}
}

func TestAppendFloatToDecimal(t *testing.T) {
	// no-op
	testAppendFloatToDecimal(t, []float64{}, nil, 0)
	testAppendFloatToDecimal(t, []float64{0}, []int64{0}, 0)
	testAppendFloatToDecimal(t, []float64{0, 1, -1, 12345678, -123456789}, []int64{0, 1, -1, 12345678, -123456789}, 0)

	// upExp
	testAppendFloatToDecimal(t, []float64{-24, 0, 4.123, 0.3}, []int64{-24000, 0, 4123, 300}, -3)
	testAppendFloatToDecimal(t, []float64{0, 10.23456789, 1e2, 1e-3, 1e-4}, []int64{0, 1023456789, 1e10, 1e5, 1e4}, -8)

	// downExp
	testAppendFloatToDecimal(t, []float64{3e17, 7e-2, 5e-7, 45, 7e-1}, []int64{3e18, 0, 0, 450, 7}, -1)
	testAppendFloatToDecimal(t, []float64{3e18, 1, 0.1, 13}, []int64{3e18, 1, 0, 13}, 0)
}

func testAppendFloatToDecimal(t *testing.T, fa []float64, daExpected []int64, eExpected int16) {
	t.Helper()

	da, e := AppendFloatToDecimal(nil, fa)
	if e != eExpected {
		t.Fatalf("unexpected e for fa=%f; got %d; expecting %d", fa, e, eExpected)
	}
	if !reflect.DeepEqual(da, daExpected) {
		t.Fatalf("unexpected da for fa=%f; got\n%d; expecting\n%d", fa, da, daExpected)
	}

	daPrefix := []int64{1, 2, 3}
	da, e = AppendFloatToDecimal(daPrefix, fa)
	if e != eExpected {
		t.Fatalf("unexpected e for fa=%f; got %d; expecting %d", fa, e, eExpected)
	}
	if !reflect.DeepEqual(da[:len(daPrefix)], daPrefix) {
		t.Fatalf("unexpected daPrefix for fa=%f; got\n%d; expecting\n%d", fa, da[:len(daPrefix)], daPrefix)
	}
	if daExpected == nil {
		daExpected = []int64{}
	}
	if !reflect.DeepEqual(da[len(daPrefix):], daExpected) {
		t.Fatalf("unexpected da for fa=%f; got\n%d; expecting\n%d", fa, da[len(daPrefix):], daExpected)
	}
}

func TestFloatToDecimal(t *testing.T) {
	testFloatToDecimal(t, 0, 0, 0)
	testFloatToDecimal(t, 1, 1, 0)
	testFloatToDecimal(t, -1, -1, 0)
	testFloatToDecimal(t, 0.9, 9, -1)
	testFloatToDecimal(t, 0.99, 99, -2)
	testFloatToDecimal(t, 9, 9, 0)
	testFloatToDecimal(t, 99, 99, 0)
	testFloatToDecimal(t, 20, 2, 1)
	testFloatToDecimal(t, 100, 1, 2)
	testFloatToDecimal(t, 3000, 3, 3)

	testFloatToDecimal(t, 0.123, 123, -3)
	testFloatToDecimal(t, -0.123, -123, -3)
	testFloatToDecimal(t, 1.2345, 12345, -4)
	testFloatToDecimal(t, -1.2345, -12345, -4)
	testFloatToDecimal(t, 12000, 12, 3)
	testFloatToDecimal(t, -12000, -12, 3)
	testFloatToDecimal(t, 1e-30, 1, -30)
	testFloatToDecimal(t, -1e-30, -1, -30)
	testFloatToDecimal(t, 1e-260, 1, -260)
	testFloatToDecimal(t, -1e-260, -1, -260)
	testFloatToDecimal(t, 321e260, 321, 260)
	testFloatToDecimal(t, -321e260, -321, 260)
	testFloatToDecimal(t, 1234567890123, 1234567890123, 0)
	testFloatToDecimal(t, -1234567890123, -1234567890123, 0)
	testFloatToDecimal(t, 123e5, 123, 5)
	testFloatToDecimal(t, 15e18, 15, 18)

	testFloatToDecimal(t, math.Inf(1), vInfPos, 0)
	testFloatToDecimal(t, math.Inf(-1), vInfNeg, 0)
	testFloatToDecimal(t, 1<<63-1, 922337203685, 7)
	testFloatToDecimal(t, -1<<63, -922337203685, 7)
}

func testFloatToDecimal(t *testing.T, f float64, vExpected int64, eExpected int16) {
	t.Helper()

	v, e := FromFloat(f)
	if v != vExpected {
		t.Fatalf("unexpected v for f=%e; got %d; expecting %d", f, v, vExpected)
	}
	if e != eExpected {
		t.Fatalf("unexpected e for f=%e; got %d; expecting %d", f, e, eExpected)
	}
}

func TestFloatToDecimalRoundtrip(t *testing.T) {
	testFloatToDecimalRoundtrip(t, 0)
	testFloatToDecimalRoundtrip(t, 1)
	testFloatToDecimalRoundtrip(t, 0.123)
	testFloatToDecimalRoundtrip(t, 1.2345)
	testFloatToDecimalRoundtrip(t, 12000)
	testFloatToDecimalRoundtrip(t, 1e-30)
	testFloatToDecimalRoundtrip(t, 1e-260)
	testFloatToDecimalRoundtrip(t, 321e260)
	testFloatToDecimalRoundtrip(t, 1234567890123)
	testFloatToDecimalRoundtrip(t, 12.34567890125)
	testFloatToDecimalRoundtrip(t, 15e18)

	testFloatToDecimalRoundtrip(t, math.Inf(1))
	testFloatToDecimalRoundtrip(t, math.Inf(-1))
	testFloatToDecimalRoundtrip(t, 1<<63-1)
	testFloatToDecimalRoundtrip(t, -1<<63)

	for i := 0; i < 1e4; i++ {
		f := rand.NormFloat64()
		testFloatToDecimalRoundtrip(t, f)
		testFloatToDecimalRoundtrip(t, f*1e-6)
		testFloatToDecimalRoundtrip(t, f*1e6)

		testFloatToDecimalRoundtrip(t, roundFloat(f, 20))
		testFloatToDecimalRoundtrip(t, roundFloat(f, 10))
		testFloatToDecimalRoundtrip(t, roundFloat(f, 5))
		testFloatToDecimalRoundtrip(t, roundFloat(f, 0))
		testFloatToDecimalRoundtrip(t, roundFloat(f, -5))
		testFloatToDecimalRoundtrip(t, roundFloat(f, -10))
		testFloatToDecimalRoundtrip(t, roundFloat(f, -20))
	}
}

func roundFloat(f float64, exp int) float64 {
	f *= math.Pow10(-exp)
	return math.Trunc(f) * math.Pow10(exp)
}

func testFloatToDecimalRoundtrip(t *testing.T, f float64) {
	t.Helper()

	v, e := FromFloat(f)
	fNew := ToFloat(v, e)
	if !equalFloat(fNew, f) {
		t.Fatalf("unexpected fNew for v=%d, e=%d; got %g; expecting %g", v, e, fNew, f)
	}

	v, e = FromFloat(-f)
	fNew = ToFloat(v, e)
	if !equalFloat(fNew, -f) {
		t.Fatalf("unexepcted fNew for v=%d, e=%d; got %g; expecting %g", v, e, fNew, -f)
	}
}

func equalFloat(f1, f2 float64) bool {
	if math.IsInf(f1, 0) {
		return math.IsInf(f1, 1) == math.IsInf(f2, 1) || math.IsInf(f1, -1) == math.IsInf(f2, -1)
	}
	eps := math.Abs(f1 - f2)
	return eps == 0 || eps*conversionPrecision < math.Abs(f1)+math.Abs(f2)
}
