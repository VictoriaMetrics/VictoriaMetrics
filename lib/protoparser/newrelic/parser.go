package newrelic

import (
	"fmt"
	"sync"
	"unicode"

	"github.com/valyala/fastjson"
	"github.com/valyala/fastjson/fastfloat"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var baseEventKeys = map[string]struct{}{
	"timestamp": {}, "eventType": {},
}

type tagsBuffer struct {
	tags []Tag
}

var tagsPool = sync.Pool{
	New: func() interface{} {
		return &tagsBuffer{tags: make([]Tag, 0)}
	},
}

// NewRelic agent sends next struct to the collector
// MetricPost entity item for the HTTP post to be sent to the ingest service.
// type MetricPost struct {
// 	ExternalKeys []string          `json:"ExternalKeys,omitempty"`
// 	EntityID     uint64            `json:"EntityID,omitempty"`
// 	IsAgent      bool              `json:"IsAgent"`
// 	Events       []json.RawMessage `json:"Events"`
// 	// Entity ID of the reporting agent, which will = EntityID when IsAgent == true.
// 	// The field is required in the backend for host metadata matching of the remote entities
// 	ReportingAgentID uint64 `json:"ReportingAgentID,omitempty"`
// }
// We are using only Events field because it contains all needed metrics

// Events represents Metrics collected from NewRelic MetricPost request
// https://docs.newrelic.com/docs/infrastructure/manage-your-data/data-instrumentation/default-infrastructure-monitoring-data/#infrastructure-events
type Events struct {
	Metrics []Metric
}

// Unmarshal takes fastjson.Value and collects Metrics
func (e *Events) Unmarshal(v []*fastjson.Value) error {
	for _, value := range v {
		events := value.Get("Events")
		if events == nil {
			return fmt.Errorf("got empty Events array from request")
		}
		eventsArr, err := events.Array()
		if err != nil {
			return fmt.Errorf("error collect events: %s", err)
		}

		for _, event := range eventsArr {
			metricData, err := event.Object()
			if err != nil {
				return fmt.Errorf("error get metric data: %s", err)
			}
			var m Metric
			metrics, err := m.unmarshal(metricData)
			if err != nil {
				return fmt.Errorf("error collect metrics from Newrelic json: %s", err)
			}
			e.Metrics = append(e.Metrics, metrics...)
		}
	}

	return nil
}

// Metric represents VictoriaMetrics metrics
type Metric struct {
	Timestamp int64
	Tags      []Tag
	Metric    string
	Value     float64
}

func (m *Metric) unmarshal(o *fastjson.Object) ([]Metric, error) {
	m.reset()

	tgsBuffer := tagsPool.Get().(*tagsBuffer)
	defer func() {
		tgsBuffer.tags = tgsBuffer.tags[:0]
		tagsPool.Put(tgsBuffer)
	}()

	metrics := make([]Metric, 0, o.Len())
	rawTs := o.Get("timestamp")
	if rawTs != nil {
		ts, err := getFloat64(rawTs)
		if err != nil {
			return nil, fmt.Errorf("invalid `timestamp` in %s: %w", o, err)
		}
		m.Timestamp = int64(ts * 1e3)
	} else {
		// Allow missing timestamp. It should be automatically populated
		// with the current time by the caller.
		m.Timestamp = 0
	}

	eventType := o.Get("eventType")
	if eventType == nil {
		return nil, fmt.Errorf("error get eventType from Events object: %s", o)
	}
	prefix := bytesutil.ToUnsafeString(eventType.GetStringBytes())
	prefix = camelToSnakeCase(prefix)

	o.Visit(func(key []byte, v *fastjson.Value) {

		k := bytesutil.ToUnsafeString(key)
		// skip base event keys which should have been parsed before this
		if _, ok := baseEventKeys[k]; ok {
			return
		}

		switch v.Type() {
		case fastjson.TypeString:
			// this is label-value pair
			value := v.Get()
			if value == nil {
				logger.Errorf("failed to get label value from NewRelic json: %s", v)
				return
			}
			name := camelToSnakeCase(k)
			val := bytesutil.ToUnsafeString(value.GetStringBytes())
			tgsBuffer.tags = append(tgsBuffer.tags, Tag{Key: name, Value: val})
		case fastjson.TypeNumber:
			// this is metric name with value
			metricName := camelToSnakeCase(k)
			if prefix != "" {
				metricName = fmt.Sprintf("%s_%s", prefix, metricName)
			}
			f, err := getFloat64(v)
			if err != nil {
				logger.Errorf("failed to get value for NewRelic metric %q: %w", k, err)
				return
			}
			metrics = append(metrics, Metric{Metric: metricName, Value: f})
		default:
			// unknown type
			logger.Errorf("got unsupported NewRelic json %s field type: %s", v, v.Type())
			return
		}
	})

	for i := range metrics {
		metrics[i].Timestamp = m.Timestamp
		metrics[i].Tags = tgsBuffer.tags
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

func camelToSnakeCase(str string) string {
	str = promrelabel.SanitizeLabelName(str)
	length := len(str)
	snakeCase := make([]byte, 0, length*2)
	tokens := make([]byte, 0, length)
	var allTokensUpper bool

	flush := func(tokens []byte) {
		for _, c := range tokens {
			snakeCase = append(snakeCase, byte(unicode.ToLower(rune(c))))
		}
	}

	for i := 0; i < length; i++ {
		char := str[i]
		if unicode.IsUpper(rune(char)) {
			switch {
			case len(tokens) == 0:
				allTokensUpper = true
				tokens = append(tokens, char)
			case allTokensUpper:
				tokens = append(tokens, char)
			default:
				flush(tokens)
				snakeCase = append(snakeCase, '_')
				tokens = tokens[:0]
				tokens = append(tokens, char)
				allTokensUpper = true
			}
			continue
		}

		switch {
		case len(tokens) == 1:
			tokens = append(tokens, char)
			allTokensUpper = false
		case allTokensUpper:
			tail := tokens[:len(tokens)-1]
			last := tokens[len(tokens)-1:]
			flush(tail)
			snakeCase = append(snakeCase, '_')
			tokens = tokens[:0]
			tokens = append(tokens, last...)
			tokens = append(tokens, char)
			allTokensUpper = false
		default:
			tokens = append(tokens, char)
		}
	}

	if len(tokens) > 0 {
		flush(tokens)
	}
	s := bytesutil.ToUnsafeString(snakeCase)
	return s
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
