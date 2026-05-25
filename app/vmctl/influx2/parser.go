package influx2

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// toFloat64 converts any numeric value InfluxDB might return into float64.
// The v2 client gives us record.Value() as interface{}, and InfluxDB fields
// can be integers, floats, booleans, or occasionally bare strings —
// we normalize all of them so VictoriaMetrics gets a consistent float64.
// Booleans map to 1/0 so they survive as a numeric signal after migration.
func toFloat64(v any) (float64, error) {
	switch i := v.(type) {
	case json.Number:
		return i.Float64()
	case float64:
		return i, nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		return strconv.ParseFloat(i, 64)
	case bool:
		if i {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("unexpected value type %T: %v", v, v)
	}
}

// fluxTimeToMillis converts a time.Time to milliseconds since Unix epoch.
// VictoriaMetrics expects ms timestamps. The v2 client already gives us
// time.Time from record.Time(), so we just divide nanoseconds by 1e6.
// Sub-millisecond precision is lost, but VM doesn't store it anyway.
func fluxTimeToMillis(t interface{ UnixNano() int64 }) int64 {
	return t.UnixNano() / 1e6
}
