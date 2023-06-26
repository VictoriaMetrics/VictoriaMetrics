package pb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// FormatString formats strings
func (x *AnyValue) FormatString() string {
	switch v := x.Value.(type) {
	case *AnyValue_StringValue:
		return v.StringValue

	case *AnyValue_BoolValue:
		return strconv.FormatBool(v.BoolValue)

	case *AnyValue_DoubleValue:
		return float64AsString(v.DoubleValue)

	case *AnyValue_IntValue:
		return strconv.FormatInt(v.IntValue, 10)

	case *AnyValue_KvlistValue:
		jsonStr, _ := json.Marshal(v.KvlistValue.Values)
		return string(jsonStr)

	case *AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(v.BytesValue)

	case *AnyValue_ArrayValue:
		jsonStr, _ := json.Marshal(v.ArrayValue.Values)
		return string(jsonStr)

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

// Reset resets fields
// it allows reusing objects without memory allocations
func (m *ExportMetricsServiceRequest) Reset() {
	m.unknownFields = m.unknownFields[:0]
	for i := range m.ResourceMetrics {
		m.ResourceMetrics[i].Reset()
	}
	m.ResourceMetrics = m.ResourceMetrics[:0]
}

// Reset resets fields
// it allows reusing objects without memory allocations
func (m *ResourceMetrics) Reset() {
	m.unknownFields = m.unknownFields[:0]
	m.Resource = nil
	for i := range m.ScopeMetrics {
		m.ScopeMetrics[i].Reset()
	}
	m.ScopeMetrics = m.ScopeMetrics[:0]
	m.SchemaUrl = ""
}

// Reset resets fields
// it allows reusing objects without memory allocations
func (m *ScopeMetrics) Reset() {
	m.unknownFields = nil
	m.Scope = nil
	for i := range m.Metrics {
		m.Metrics[i] = nil
	}
	m.Metrics = m.Metrics[:0]
	m.SchemaUrl = ""
}
