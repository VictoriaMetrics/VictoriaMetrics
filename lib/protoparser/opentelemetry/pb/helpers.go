package pb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// FormatString returns string reperesentation for av.
func (av *AnyValue) FormatString() string {
	if av == nil {
		return ""
	}
	switch {
	case av.StringValue != nil:
		return *av.StringValue
	case av.BoolValue != nil:
		return strconv.FormatBool(*av.BoolValue)
	case av.IntValue != nil:
		return strconv.FormatInt(*av.IntValue, 10)
	case av.DoubleValue != nil:
		return float64AsString(*av.DoubleValue)
	case av.ArrayValue != nil:
		jsonStr, _ := json.Marshal(av.ArrayValue.Values)
		return string(jsonStr)
	case av.KeyValueList != nil:
		jsonStr, _ := json.Marshal(av.KeyValueList.Values)
		return string(jsonStr)
	case av.BytesValue != nil:
		return base64.StdEncoding.EncodeToString(*av.BytesValue)
	default:
		return ""
	}
}

func float64AsString(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Sprintf("json: unsupported value: %s", strconv.FormatFloat(f, 'g', -1, 64))
	}

	// Convert as if by ES6 number to string conversion.
	// This matches most other JSON generators.
	// See golang.org/issue/6384 and golang.org/issue/14135.
	// Like fmt %g, but the exponent cutoffs are different
	// and exponents themselves are not padded to two digits.
	scratch := [64]byte{}
	b := scratch[:0]
	abs := math.Abs(f)
	fmt := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		fmt = 'e'
	}
	b = strconv.AppendFloat(b, f, fmt, -1, 64)
	if fmt == 'e' {
		// clean up e-09 to e-9
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return string(b)
}
