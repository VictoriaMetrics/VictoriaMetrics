package decimal

import (
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
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
	for i, v := range a {
		adjExp := upExp
		for adjExp > 0 {
			v *= 10
			adjExp--
		}
		a[i] = v
	}
	if downExp > 0 {
		for i, v := range b {
			adjExp := downExp
			for adjExp > 0 {
				v /= 10
				adjExp--
			}
			b[i] = v
		}
	}
	return be + downExp
}

// ExtendFloat64sCapacity extends dst capacity to hold additionalItems
// and returns the extended dst.
func ExtendFloat64sCapacity(dst []float64, additionalItems int) []float64 {
	dstLen := len(dst)
	if n := dstLen + additionalItems - cap(dst); n > 0 {
		dst = append(dst[:cap(dst)], make([]float64, n)...)
	}
	return dst[:dstLen]
}

// ExtendInt64sCapacity extends dst capacity to hold additionalItems
// and returns the extended dst.
func ExtendInt64sCapacity(dst []int64, additionalItems int) []int64 {
	dstLen := len(dst)
	if n := dstLen + additionalItems - cap(dst); n > 0 {
		dst = append(dst[:cap(dst)], make([]int64, n)...)
	}
	return dst[:dstLen]
}

// AppendDecimalToFloat converts each item in va to f=v*10^e, appends it
// to dst and returns the resulting dst.
func AppendDecimalToFloat(dst []float64, va []int64, e int16) []float64 {
	// Extend dst capacity in order to eliminate memory allocations below.
	dst = ExtendFloat64sCapacity(dst, len(va))

	if fastnum.IsInt64Zeros(va) {
		return fastnum.AppendFloat64Zeros(dst, len(va))
	}
	if e == 0 {
		if fastnum.IsInt64Ones(va) {
			return fastnum.AppendFloat64Ones(dst, len(va))
		}
		for _, v := range va {
			f := float64(v)
			dst = append(dst, f)
		}
		return dst
	}

	// increase conversion precision for negative exponents by dividing by e10
	if e < 0 {
		e10 := math.Pow10(int(-e))
		for _, v := range va {
			f := float64(v) / e10
			dst = append(dst, f)
		}
		return dst
	}
	e10 := math.Pow10(int(e))
	for _, v := range va {
		f := float64(v) * e10
		dst = append(dst, f)
	}
	return dst
}

// AppendFloatToDecimal converts each item in src to v*10^e and appends
// each v to dst returning it as va.
//
// It tries minimizing each item in dst.
func AppendFloatToDecimal(dst []int64, src []float64) (va []int64, e int16) {
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

	// Extend dst capacity in order to eliminate memory allocations below.
	dst = ExtendInt64sCapacity(dst, len(src))

	vaev := vaeBufPool.Get()
	if vaev == nil {
		vaev = &vaeBuf{
			va: make([]int64, len(src)),
			ea: make([]int16, len(src)),
		}
	}
	vae := vaev.(*vaeBuf)
	vae.va = vae.va[:0]
	vae.ea = vae.ea[:0]

	// Determine the minimum exponent across all src items.
	v, exp := FromFloat(src[0])
	vae.va = append(vae.va, v)
	vae.ea = append(vae.ea, exp)
	minExp := exp
	for _, f := range src[1:] {
		v, exp := FromFloat(f)
		vae.va = append(vae.va, v)
		vae.ea = append(vae.ea, exp)
		if exp < minExp {
			minExp = exp
		}
	}

	// Determine whether all the src items may be upscaled to minExp.
	// If not, adjust minExp accordingly.
	downExp := int16(0)
	for i, v := range vae.va {
		exp := vae.ea[i]
		upExp := exp - minExp
		maxUpExp := maxUpExponent(v)
		if upExp-maxUpExp > downExp {
			downExp = upExp - maxUpExp
		}
	}
	minExp += downExp

	// Scale each item in src to minExp and append it to dst.
	for i, v := range vae.va {
		exp := vae.ea[i]
		adjExp := exp - minExp
		for adjExp > 0 {
			v *= 10
			adjExp--
		}
		for adjExp < 0 {
			v /= 10
			adjExp++
		}
		dst = append(dst, v)
	}

	vaeBufPool.Put(vae)

	return dst, minExp
}

type vaeBuf struct {
	va []int64
	ea []int16
}

var vaeBufPool sync.Pool

func maxUpExponent(v int64) int16 {
	if v == 0 {
		// Any exponent allowed.
		return 1024
	}
	if v < 0 {
		v = -v
	}
	if v < 0 {
		// Handle corner case for v=-1<<63
		return 0
	}

	maxMultiplier := ((1 << 63) - 1) / uint64(v)
	switch {
	case maxMultiplier >= 1e19:
		return 19
	case maxMultiplier >= 1e18:
		return 18
	case maxMultiplier >= 1e17:
		return 17
	case maxMultiplier >= 1e16:
		return 16
	case maxMultiplier >= 1e15:
		return 15
	case maxMultiplier >= 1e14:
		return 14
	case maxMultiplier >= 1e13:
		return 13
	case maxMultiplier >= 1e12:
		return 12
	case maxMultiplier >= 1e11:
		return 11
	case maxMultiplier >= 1e10:
		return 10
	case maxMultiplier >= 1e9:
		return 9
	case maxMultiplier >= 1e8:
		return 8
	case maxMultiplier >= 1e7:
		return 7
	case maxMultiplier >= 1e6:
		return 6
	case maxMultiplier >= 1e5:
		return 5
	case maxMultiplier >= 1e4:
		return 4
	case maxMultiplier >= 1e3:
		return 3
	case maxMultiplier >= 1e2:
		return 2
	case maxMultiplier >= 1e1:
		return 1
	default:
		return 0
	}
}

// ToFloat returns f=v*10^e.
func ToFloat(v int64, e int16) float64 {
	f := float64(v)
	// increase conversion precision for negative exponents by dividing by e10
	if e < 0 {
		return f / math.Pow10(int(-e))
	}
	return f * math.Pow10(int(e))
}

const (
	vInfPos = 1<<63 - 1
	vInfNeg = -1 << 63

	vMax = 1<<63 - 3
	vMin = -1<<63 + 1
)

// FromFloat converts f to v*10^e.
//
// It tries minimizing v.
// For instance, for f = -1.234 it returns v = -1234, e = -3.
//
// FromFloat doesn't work properly with NaN values, so don't pass them here.
func FromFloat(f float64) (int64, int16) {
	if f == 0 {
		return 0, 0
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
