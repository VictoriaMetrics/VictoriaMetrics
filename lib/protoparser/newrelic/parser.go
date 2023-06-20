package newrelic

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"
)

type Events struct {
	Metrics []*Metric
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
			var m Metric
			metrics, err := m.unmarshal(metricData)
			if err != nil {
				return fmt.Errorf("error collect metrics from Newrelic json: %s", err)
			}
			e.Metrics = metrics
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

func (m *Metric) unmarshal(o *fastjson.Object) ([]*Metric, error) {
	m.reset()
	metricNames := make(map[string]float64)
	var tags []Tag
	ts, err := o.Get("timestamp").Int64()
	if err != nil {
		return nil, err
	}
	m.Timestamp = ts * 1e3

	partOfMetricName := o.Get("eventType").GetStringBytes()
	entity := o.Get("entityKey").GetStringBytes()
	tags = append(tags, Tag{Key: "entityKey", Value: string(entity)})

	o.Visit(func(key []byte, v *fastjson.Value) {
		k := string(key)
		// skip already parsed values
		if contains(k) {
			return
		}

		switch v.Type() {
		case fastjson.TypeString:
			// this is label with value
			name := string(key)
			value := v.Get().GetStringBytes()
			tags = append(tags, Tag{Key: name, Value: string(value)})
		case fastjson.TypeNumber:
			// this is metric name with value
			metricName := camelToSnakeCase(fmt.Sprintf("%s_%s", partOfMetricName, string(key)))
			f, err := getFloat64(v)
			if err != nil {
				logger.Errorf("error get Newrelic value for metric: %q; %s", string(key), err)
				return
			}
			metricNames[metricName] = f
		default:
			// unknown type
			logger.Errorf("got unsupported Newrelic json type: %s", v.Type())
			return
		}
	})

	metrics := make([]*Metric, len(metricNames))

	for name, value := range metricNames {
		m.Metric = name
		m.Tags = tags
		m.Value = value
		metrics = append(metrics, m)
	}

	return metrics, nil
}

func (m *Metric) reset() {
	m.Timestamp = 0
	m.Tags = nil
	m.Metric = ""
	m.Value = 0
}

// Tag is an NewRelic tag.
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

func contains(key string) bool {
	// this is keys of BaseEvent type.
	// this type contains all NewRelic structs
	baseEventKeys := []string{"timestamp", "entityKey", "eventType"}
	for _, baseKey := range baseEventKeys {
		if baseKey == key {
			return true
		}
	}
	return false
}
