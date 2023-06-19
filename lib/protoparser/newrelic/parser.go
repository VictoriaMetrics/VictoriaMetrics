package newrelic

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
)

type Events struct {
	Metrics []Metric
}

func (e *Events) Unmarshal(v []*fastjson.Value) error {
	for _, value := range v {
		events, err := value.Get("Events").Array()
		if err != nil {
			return fmt.Errorf("error collect events: %s", err)
		}

		for _, event := range events {
			metricData, err := event.Object()
			if err != nil {
				return fmt.Errorf("error get metric data: %s", err)
			}

			var metric Metric
			metricNames := make(map[string]float64)
			var tags []Tag
			ts, err := metricData.Get("timestamp").Int64()
			if err != nil {
				return err
			}
			metric.Timestamp = ts * 1e3
			partOfMetricName := metricData.Get("eventType").GetStringBytes()
			entity := metricData.Get("entityKey").GetStringBytes()
			tags = append(tags, Tag{Key: "entityKey", Value: string(entity)})

			metricData.Visit(func(key []byte, v *fastjson.Value) {
				if bytes.Equal(key, []byte("timestamp")) ||
					bytes.Equal(key, []byte("entityKey")) ||
					bytes.Equal(key, []byte("eventType")) {
					return
				}

				switch v.Type() {
				case fastjson.TypeString:
					// this metric is label
					name := camelToSnakeCase(string(key))
					value := v.Get().GetStringBytes()
					tags = append(tags, Tag{Key: name, Value: string(value)})
				case fastjson.TypeNumber:
					// this is metric value
					metricName := camelToSnakeCase(fmt.Sprintf("%s_%s", partOfMetricName, string(key)))
					f, err := getFloat64(v)
					if err != nil {
						return
					}
					metricNames[metricName] = f
				default:
					// unknown type
					return
				}
			})
			for name, value := range metricNames {
				metric.Metric = name
				metric.Tags = tags
				metric.Value = value
				e.Metrics = append(e.Metrics, metric)
			}
		}
	}

	return nil
}

type Metric struct {
	Timestamp int64
	Tags      []Tag
	Metric    string
	Value     float64
}

func (m *Metric) reset() {
	m.Timestamp = 0
	m.Tags = nil
	m.Metric = ""
	m.Value = 0
}

// Tag is an OpenTSDB tag.
type Tag struct {
	Key   string
	Value string
}

func (t *Tag) reset() {
	t.Key = ""
	t.Value = ""
}

func camelToSnakeCase(camelCase string) string {
	var str strings.Builder

	for i, char := range camelCase {
		if i > 0 && unicode.IsUpper(char) {
			str.WriteRune('_')
		}
		str.WriteRune(unicode.ToLower(char))
	}

	return str.String()
}

func getFloat64(v *fastjson.Value) (float64, error) {
	switch v.Type() {
	case fastjson.TypeNumber:
		return v.Float64()
	case fastjson.TypeString:
		vStr, _ := v.StringBytes()
		vFloat, err := fastfloat.Parse(bytesutil.ToUnsafeString(vStr))
		if err != nil {
			return 0, fmt.Errorf("cannot parse value %q: %w", vStr, err)
		}
		return vFloat, nil
	default:
		return 0, fmt.Errorf("value doesn't contain float64; it contains %s", v.Type())
	}
}
