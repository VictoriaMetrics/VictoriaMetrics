package decimal

import (
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// CalibrateScale calibrates a and b with the corresponding exponents ae, be
// and returns the resulting exponent e.
func CalibrateScale(a []int64, ae int16, b []int64, be int16) (e int16) {
	if ae == be {
		// Fast path - exponents are equal.
		return ae
	}
	if len(a) == 0 {
		return be
	}
	if len(b) == 0 {
		return ae
	}

	if ae < be {
		a, b = b, a
		ae, be = be, ae
	}

	upExp := ae - be
	downExp := int16(0)
	for _, v := range a {
		maxUpExp := maxUpExponent(v)
		if upExp-maxUpExp > downExp {
			downExp = upExp - maxUpExp
		}
	}
	upExp -= downExp

	if upExp > 0 {
		m := getDecimalMultiplier(uint16(upExp))
		for i, v := range a {
			if isSpecialValue(v) {
				// Do not take into account special values.
				continue
			}
			a[i] = v * m
		}
	}
	if downExp > 0 {
		if downExp > 18 {
			for i, v := range b {
				if isSpecialValue(v) {
					// Do not take into account special values.
					continue
				}
				b[i] = 0
			}
		} else {
			m := getDecimalMultiplier(uint16(downExp))
			for i, v := range b {
				if isSpecialValue(v) {
					// Do not take into account special values.
					continue
				}
				b[i] = v / m
			}
		}
	}
	return be + downExp
}

func getDecimalMultiplier(exp uint16) int64 {
	if exp >= uint16(len(decimalMultipliers)) {
		return 1
	}
	return decimalMultipliers[exp]
}

var decimalMultipliers = []int64{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18}

// ExtendFloat64sCapacity extends dst capacity to hold additionalItems
// and returns the extended dst.
func ExtendFloat64sCapacity(dst []float64, additionalItems int) []float64 {
	return slicesutil.ExtendCapacity(dst, additionalItems)
}

// ExtendInt64sCapacity extends dst capacity to hold additionalItems
// and returns the extended dst.
func ExtendInt64sCapacity(dst []int64, additionalItems int) []int64 {
	return slicesutil.ExtendCapacity(dst, additionalItems)
}

func extendInt16sCapacity(dst []int16, additionalItems int) []int16 {
	return slicesutil.ExtendCapacity(dst, additionalItems)
}

// AppendDecimalToFloat converts each item in va to f=v*10^e, appends it
// to dst and returns the resulting dst.
func AppendDecimalToFloat(dst []float64, va []int64, e int16) []float64 {
	// Extend dst capacity in order to eliminate memory allocations below.
	dst = ExtendFloat64sCapacity(dst, len(va))
	a := dst[len(dst) : len(dst)+len(va)]

	if fastnum.IsInt64Zeros(va) {
		return fastnum.AppendFloat64Zeros(dst, len(va))
	}
	if e == 0 {
		if fastnum.IsInt64Ones(va) {
			return fastnum.AppendFloat64Ones(dst, len(va))
		}
		_ = a[len(va)-1]
		for i, v := range va {
			a[i] = float64(v)
			if !isSpecialValue(v) {
				continue
			}
			switch v {
			case vInfPos:
				a[i] = infPos
			case vInfNeg:
				a[i] = infNeg
			default:
				a[i] = StaleNaN
			}
		}
		return dst[:len(dst)+len(va)]
	}

	// increase conversion precision for negative exponents by dividing by e10
	if e < 0 {
		e10 := math.Pow10(int(-e))
		_ = a[len(va)-1]
		for i, v := range va {
			a[i] = float64(v) / e10
			if !isSpecialValue(v) {
				continue
			}
			switch v {
			case vInfPos:
				a[i] = infPos
			case vInfNeg:
				a[i] = infNeg
			default:
				a[i] = StaleNaN
			}
		}
		return dst[:len(dst)+len(va)]
	}
	e10 := math.Pow10(int(e))
	_ = a[len(va)-1]
	for i, v := range va {
		a[i] = float64(v) * e10
		if !isSpecialValue(v) {
			continue
		}
		switch v {
		case vInfPos:
			a[i] = infPos
		case vInfNeg:
			a[i] = infNeg
		default:
			a[i] = StaleNaN
		}
	}
	return dst[:len(dst)+len(va)]
}

// AppendFloatToDecimal converts each item in src to v*10^e and appends
// each v to dst returning it as va.
//
// It tries minimizing each item in dst.
func AppendFloatToDecimal(dst []int64, src []float64) ([]int64, int16) {
	if len(src) == 0 {
		return dst, 0
	}
	if fastnum.IsFloat64Zeros(src) {
		dst = fastnum.AppendInt64Zeros(dst, len(src))
		return dst, 0
	}
	if fastnum.IsFloat64Ones(src) {
		dst = fastnum.AppendInt64Ones(dst, len(src))
		return dst, 0
	}

	vaev := vaeBufPool.Get()
	if vaev == nil {
		vaev = &vaeBuf{
			va: make([]int64, len(src)),
			ea: make([]int16, len(src)),
		}
	}
	vae := vaev.(*vaeBuf)
	va := vae.va[:0]
	ea := vae.ea[:0]
	va = ExtendInt64sCapacity(va, len(src))
	va = va[:len(src)]
	ea = extendInt16sCapacity(ea, len(src))
	ea = ea[:len(src)]

	// Determine the minimum exponent across all src items.
	minExp := int16(1<<15 - 1)
	for i, f := range src {
		v, exp := FromFloat(f)
		va[i] = v
		ea[i] = exp
		if exp < minExp && !isSpecialValue(v) {
			minExp = exp
		}
	}

	// Determine whether all the src items may be upscaled to minExp.
	// If not, adjust minExp accordingly.
	downExp := int16(0)
	_ = ea[len(va)-1]
	for i, v := range va {
		exp := ea[i]
		upExp := exp - minExp
		maxUpExp := maxUpExponent(v)
		if upExp-maxUpExp > downExp {
			downExp = upExp - maxUpExp
		}
	}
	minExp += downExp

	// Extend dst capacity in order to eliminate memory allocations below.
	dst = ExtendInt64sCapacity(dst, len(src))
	a := dst[len(dst) : len(dst)+len(src)]

	// Scale each item in src to minExp and append it to dst.
	_ = a[len(va)-1]
	_ = ea[len(va)-1]
	for i, v := range va {
		if isSpecialValue(v) {
			// There is no need in scaling special values.
			a[i] = v
			continue
		}
		exp := ea[i]
		adjExp := exp - minExp
		for adjExp > 0 {
			v *= 10
			adjExp--
		}
		for adjExp < 0 {
			v /= 10
			adjExp++
		}
		a[i] = v
	}

	vae.va = va
	vae.ea = ea
	vaeBufPool.Put(vae)

	return dst[:len(dst)+len(va)], minExp
}

type vaeBuf struct {
	va []int64
	ea []int16
}

var vaeBufPool sync.Pool

const int64Max = int64(1<<63 - 1)

func maxUpExponent(v int64) int16 {
	if v == 0 || isSpecialValue(v) {
		// Any exponent allowed for zeroes and special values.
		return 1024
	}
	if v < 0 {
		v = -v
	}
	if v < 0 {
		// Handle corner case for v=-1<<63
		return 0
	}
	switch {
	case v <= int64Max/1e18:
		return 18
	case v <= int64Max/1e17:
		return 17
	case v <= int64Max/1e16:
		return 16
	case v <= int64Max/1e15:
		return 15
	case v <= int64Max/1e14:
		return 14
	case v <= int64Max/1e13:
		return 13
	case v <= int64Max/1e12:
		return 12
	case v <= int64Max/1e11:
		return 11
	case v <= int64Max/1e10:
		return 10
	case v <= int64Max/1e9:
		return 9
	case v <= int64Max/1e8:
		return 8
	case v <= int64Max/1e7:
		return 7
	case v <= int64Max/1e6:
		return 6
	case v <= int64Max/1e5:
		return 5
	case v <= int64Max/1e4:
		return 4
	case v <= int64Max/1e3:
		return 3
	case v <= int64Max/1e2:
		return 2
	case v <= int64Max/1e1:
		return 1
	default:
		return 0
	}
}

// RoundToDecimalDigits rounds f to the given number of decimal digits after the point.
//
// See also RoundToSignificantFigures.
func RoundToDecimalDigits(f float64, digits int) float64 {
	if IsStaleNaN(f) {
		// Do not modify stale nan mark value.
		return f
	}
	if digits <= -100 || digits >= 100 {
		return f
	}
	m := math.Pow10(digits)
	return math.Round(f*m) / m
}

// RoundToSignificantFigures rounds f to value with the given number of significant figures.
//
// See also RoundToDecimalDigits.
func RoundToSignificantFigures(f float64, digits int) float64 {
	if IsStaleNaN(f) {
		// Do not modify stale nan mark value.
		return f
	}
	if digits <= 0 || digits >= 18 {
		return f
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f == 0 {
		return f
	}
	n := int64(math.Pow10(digits))
	isNegative := f < 0
	if isNegative {
		f = -f
	}
	v, e := positiveFloatToDecimal(f)
	if v > vMax {
		v = vMax
	}
	var rem int64
	for v > n {
		rem = v % 10
		v /= 10
		e++
	}
	if rem >= 5 {
		v++
	}
	if isNegative {
		v = -v
	}
	return ToFloat(v, e)
}

// ToFloat returns f=v*10^e.
func ToFloat(v int64, e int16) float64 {
	if isSpecialValue(v) {
		if v == vInfPos {
			return infPos
		}
		if v == vInfNeg {
			return infNeg
		}
		return StaleNaN
	}
	f := float64(v)
	// increase conversion precision for negative exponents by dividing by e10
	if e < 0 {
		return f / math.Pow10(int(-e))
	}
	return f * math.Pow10(int(e))
}

var (
	infPos = math.Inf(1)
	infNeg = math.Inf(-1)
)

// StaleNaN is a special NaN value, which is used as Prometheus staleness mark.
// See https://www.robustperception.io/staleness-and-promql
var StaleNaN = math.Float64frombits(staleNaNBits)

const (
	vInfPos   = 1<<63 - 1
	vInfNeg   = -1 << 63
	vStaleNaN = 1<<63 - 2

	vMax = 1<<63 - 3
	vMin = -1<<63 + 1

	// staleNaNbits is bit representation of Prometheus staleness mark (aka stale NaN).
	// This mark is put by Prometheus at the end of time series for improving staleness detection.
	// See https://www.robustperception.io/staleness-and-promql
	staleNaNBits uint64 = 0x7ff0000000000002
)

func isSpecialValue(v int64) bool {
	return v > vMax || v < vMin
}

// IsStaleNaN returns true if f represents Prometheus staleness mark.
func IsStaleNaN(f float64) bool {
	return math.Float64bits(f) == staleNaNBits
}

// IsStaleNaNInt64 returns true if i represents Prometheus staleness mark.
func IsStaleNaNInt64(i int64) bool {
	return i == vStaleNaN
}

// FromFloat converts f to v*10^e.
//
// It tries minimizing v.
// For instance, for f = -1.234 it returns v = -1234, e = -3.
//
// FromFloat doesn't work properly with NaN values other than Prometheus staleness mark, so don't pass them here.
func FromFloat(f float64) (int64, int16) {
	if f == 0 {
		return 0, 0
	}
	if IsStaleNaN(f) {
		return vStaleNaN, 0
	}
	if math.IsInf(f, 0) {
		return fromFloatInf(f)
	}
	if f > 0 {
		v, e := positiveFloatToDecimal(f)
		if v > vMax {
			v = vMax
		}
		return v, e
	}
	v, e := positiveFloatToDecimal(-f)
	v = -v
	if v < vMin {
		v = vMin
	}
	return v, e
}

func fromFloatInf(f float64) (int64, int16) {
	// Limit infs by max and min values for int64
	if math.IsInf(f, 1) {
		return vInfPos, 0
	}
	return vInfNeg, 0
}

func positiveFloatToDecimal(f float64) (int64, int16) {
	// There is no need in checking for f == 0, since it should be already checked by the caller.
	u := uint64(f)
	if float64(u) != f {
		return positiveFloatToDecimalSlow(f)
	}
	// Fast path for integers.
	if u < 1<<55 && u%10 != 0 {
		return int64(u), 0
	}
	return getDecimalAndScale(u)
}

func getDecimalAndScale(u uint64) (int64, int16) {
	var scale int16
	for u >= 1<<55 {
		// Remove trailing garbage bits left after float64->uint64 conversion,
		// since float64 contains only 53 significant bits.
		// See https://en.wikipedia.org/wiki/Double-precision_floating-point_format
		u /= 10
		scale++
	}
	if u%10 != 0 {
		return int64(u), scale
	}
	// Minimize v by converting trailing zeros to scale.
	u /= 10
	scale++
	for u != 0 && u%10 == 0 {
		u /= 10
		scale++
	}
	return int64(u), scale
}

func positiveFloatToDecimalSlow(f float64) (int64, int16) {
	// Slow path for floating point numbers.
	var scale int16
	prec := conversionPrecision
	if f > 1e6 || f < 1e-6 {
		// Normalize f, so it is in the small range suitable
		// for the next loop.
		if f > 1e6 {
			// Increase conversion precision for big numbers.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/213
			prec = 1e15
		}
		_, exp := math.Frexp(f)
		// Bound the exponent according to https://en.wikipedia.org/wiki/Double-precision_floating-point_format
		// This fixes the issue https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1114
		if exp < -1022 {
			exp = -1022
		} else if exp > 1023 {
			exp = 1023
		}
		scale = int16(float64(exp) * (math.Ln2 / math.Ln10))
		f *= math.Pow10(-int(scale))
	}

	// Multiply f by 100 until the fractional part becomes
	// too small comparing to integer part.
	for f < prec {
		x, frac := math.Modf(f)
		if frac*prec < x {
			f = x
			break
		}
		if (1-frac)*prec < x {
			f = x + 1
			break
		}
		f *= 100
		scale -= 2
	}
	u := uint64(f)
	if u%10 != 0 {
		return int64(u), scale
	}

	// Minimize u by converting trailing zero to scale.
	u /= 10
	scale++
	return int64(u), scale
}

const conversionPrecision = 1e12
