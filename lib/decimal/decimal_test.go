package decimal

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestPositiveFloatToDecimal(t *testing.T) {
	f := func(f float64, decimalExpected int64, exponentExpected int16) {
		t.Helper()
		decimal, exponent := positiveFloatToDecimal(f)
		if decimal != decimalExpected {
			t.Fatalf("unexpected decimal for positiveFloatToDecimal(%f); got %d; want %d", f, decimal, decimalExpected)
		}
		if exponent != exponentExpected {
			t.Fatalf("unexpected exponent for positiveFloatToDecimal(%f); got %d; want %d", f, exponent, exponentExpected)
		}
	}
	f(0, 0, 1) // The exponent is 1 is OK here. See comment in positiveFloatToDecimal.
	f(1, 1, 0)
	f(30, 3, 1)
	f(12345678900000000, 123456789, 8)
	f(12345678901234567, 12345678901234568, 0)
	f(1234567890123456789, 12345678901234567, 2)
	f(12345678901234567890, 12345678901234567, 3)
	f(18446744073670737131, 18446744073670737, 3)
	f(123456789012345678901, 12345678901234568, 4)
	f(1<<53, 1<<53, 0)
	f(1<<54, 18014398509481984, 0)
	f(1<<55, 3602879701896396, 1)
	f(1<<62, 4611686018427387, 3)
	f(1<<63, 9223372036854775, 3)
	f(1<<64, 18446744073709548, 3)
	f(1<<65, 368934881474191, 5)
	f(1<<66, 737869762948382, 5)
	f(1<<67, 1475739525896764, 5)

	f(0.1, 1, -1)
	f(123456789012345678e-5, 12345678901234568, -4)
	f(1234567890123456789e-10, 12345678901234568, -8)
	f(1234567890123456789e-14, 1234567890123, -8)
	f(1234567890123456789e-17, 12345678901234, -12)
	f(1234567890123456789e-20, 1234567890123, -14)

	f(0.000874957, 874957, -9)
	f(0.001130435, 1130435, -9)
}

func TestAppendDecimalToFloat(t *testing.T) {
	testAppendDecimalToFloat(t, []int64{}, 0, nil)
	testAppendDecimalToFloat(t, []int64{0}, 0, []float64{0})
	testAppendDecimalToFloat(t, []int64{0}, 10, []float64{0})
	testAppendDecimalToFloat(t, []int64{0}, -10, []float64{0})
	testAppendDecimalToFloat(t, []int64{-1, -10, 0, 100}, 2, []float64{-1e2, -1e3, 0, 1e4})
	testAppendDecimalToFloat(t, []int64{-1, -10, 0, 100}, -2, []float64{-1e-2, -1e-1, 0, 1})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -5, []float64{8.74957, 1.130435e1})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -6, []float64{8.74957e-1, 1.130435})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -7, []float64{8.74957e-2, 1.130435e-1})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -8, []float64{8.74957e-3, 1.130435e-2})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -9, []float64{8.74957e-4, 1.130435e-3})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -10, []float64{8.74957e-5, 1.130435e-4})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -11, []float64{8.74957e-6, 1.130435e-5})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -12, []float64{8.74957e-7, 1.130435e-6})
	testAppendDecimalToFloat(t, []int64{874957, 1130435}, -13, []float64{8.74957e-8, 1.130435e-7})
	testAppendDecimalToFloat(t, []int64{vInfPos, vInfNeg, 1, 2}, 0, []float64{9.223372036854776e+18, -9.223372036854776e+18, 1, 2})
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
	testCalibrateScale(t, []int64{vInfPos, 1200}, []int64{35, 1}, 0, 40, []int64{0, 0}, []int64{35e17, 1e17}, 23)
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
	f := func(v int64, eExpected int16) {
		t.Helper()

		e := maxUpExponent(v)
		if e != eExpected {
			t.Fatalf("unexpected e for v=%d; got %d; expecting %d", v, e, eExpected)
		}
		e = maxUpExponent(-v)
		if e != eExpected {
			t.Fatalf("unexpected e for v=%d; got %d; expecting %d", -v, e, eExpected)
		}
	}

	f(vInfPos, 0)
	f(vInfNeg, 0)
	f(0, 1024)
	f(-1<<63, 0)
	f((-1<<63)+1, 0)
	f((1<<63)-1, 0)
	f(1, 18)
	f(12, 17)
	f(123, 16)
	f(1234, 15)
	f(12345, 14)
	f(123456, 13)
	f(1234567, 12)
	f(12345678, 11)
	f(123456789, 10)
	f(1234567890, 9)
	f(12345678901, 8)
	f(123456789012, 7)
	f(1234567890123, 6)
	f(12345678901234, 5)
	f(123456789012345, 4)
	f(1234567890123456, 3)
	f(12345678901234567, 2)
	f(123456789012345678, 1)
	f(1234567890123456789, 0)
	f(923456789012345678, 0)
	f(92345678901234567, 1)
	f(9234567890123456, 2)
	f(923456789012345, 3)
	f(92345678901234, 4)
	f(9234567890123, 5)
	f(923456789012, 6)
	f(92345678901, 7)
	f(9234567890, 8)
	f(923456789, 9)
	f(92345678, 10)
	f(9234567, 11)
	f(923456, 12)
	f(92345, 13)
	f(9234, 14)
	f(923, 15)
	f(92, 17)
	f(9, 18)
}

func TestAppendFloatToDecimal(t *testing.T) {
	// no-op
	testAppendFloatToDecimal(t, []float64{}, nil, 0)
	testAppendFloatToDecimal(t, []float64{0}, []int64{0}, 0)
	testAppendFloatToDecimal(t, []float64{infPos, infNeg, 123}, []int64{vInfPos, vInfNeg, 123}, 0)
	testAppendFloatToDecimal(t, []float64{infPos, infNeg, 123, 1e-4, 1e32}, []int64{92233, -92233, 0, 0, 1000000000000000000}, 14)
	testAppendFloatToDecimal(t, []float64{float64(vInfPos), float64(vInfNeg), 123}, []int64{9223372036854775000, -9223372036854775000, 123}, 0)
	testAppendFloatToDecimal(t, []float64{0, -0, 1, -1, 12345678, -123456789}, []int64{0, 0, 1, -1, 12345678, -123456789}, 0)

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
	f := func(f float64, vExpected int64, eExpected int16) {
		t.Helper()
		v, e := FromFloat(f)
		if v != vExpected {
			t.Fatalf("unexpected v for f=%e; got %d; expecting %d", f, v, vExpected)
		}
		if e != eExpected {
			t.Fatalf("unexpected e for f=%e; got %d; expecting %d", f, e, eExpected)
		}
	}

	f(0, 0, 0)
	f(1, 1, 0)
	f(-1, -1, 0)
	f(0.9, 9, -1)
	f(0.99, 99, -2)
	f(9, 9, 0)
	f(99, 99, 0)
	f(20, 2, 1)
	f(100, 1, 2)
	f(3000, 3, 3)

	f(0.123, 123, -3)
	f(-0.123, -123, -3)
	f(1.2345, 12345, -4)
	f(-1.2345, -12345, -4)
	f(12000, 12, 3)
	f(-12000, -12, 3)
	f(1e-30, 1, -30)
	f(-1e-30, -1, -30)
	f(1e-260, 1, -260)
	f(-1e-260, -1, -260)
	f(321e260, 321, 260)
	f(-321e260, -321, 260)
	f(1234567890123, 1234567890123, 0)
	f(-1234567890123, -1234567890123, 0)
	f(123e5, 123, 5)
	f(15e18, 15, 18)

	f(math.Inf(1), vInfPos, 0)
	f(math.Inf(-1), vInfNeg, 0)
	f(1<<63-1, 9223372036854775, 3)
	f(-1<<63, -9223372036854775, 3)

	// Test precision loss due to conversionPrecision.
	f(0.1234567890123456, 12345678901234, -14)
	f(-123456.7890123456, -12345678901234, -8)
}

func TestFloatToDecimalRoundtrip(t *testing.T) {
	f := func(f float64) {
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

	f(0)
	f(1)
	f(0.123)
	f(1.2345)
	f(12000)
	f(1e-30)
	f(1e-260)
	f(321e260)
	f(1234567890123)
	f(12.34567890125)
	f(-1234567.8901256789)
	f(15e18)
	f(0.000874957)
	f(0.001130435)

	f(2933434554455e245)
	f(3439234258934e-245)
	f(float64(vInfPos))
	f(float64(vInfNeg))
	f(infPos)
	f(infNeg)

	for i := 0; i < 1e4; i++ {
		v := rand.NormFloat64()
		f(v)
		f(v * 1e-6)
		f(v * 1e6)

		f(roundFloat(v, 20))
		f(roundFloat(v, 10))
		f(roundFloat(v, 5))
		f(roundFloat(v, 0))
		f(roundFloat(v, -5))
		f(roundFloat(v, -10))
		f(roundFloat(v, -20))
	}
}

func roundFloat(f float64, exp int) float64 {
	f *= math.Pow10(-exp)
	return math.Trunc(f) * math.Pow10(exp)
}

func equalFloat(f1, f2 float64) bool {
	f1 = adjustInf(f1)
	f2 = adjustInf(f2)
	if math.IsInf(f1, 0) {
		return math.IsInf(f1, 1) == math.IsInf(f2, 1) || math.IsInf(f1, -1) == math.IsInf(f2, -1)
	}
	eps := math.Abs(f1 - f2)
	return eps == 0 || eps*conversionPrecision < math.Abs(f1)+math.Abs(f2)
}

func adjustInf(f float64) float64 {
	if f == float64(vInfPos) {
		return infPos
	}
	if f == float64(vInfNeg) {
		return infNeg
	}
	return f
}

var (
	infPos = math.Inf(1)
	infNeg = math.Inf(-1)
)
