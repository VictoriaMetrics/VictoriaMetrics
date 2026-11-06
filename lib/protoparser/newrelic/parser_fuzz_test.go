package newrelic

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte(`[{"Events":[{"eventType":"SystemSample","timestamp":42,"cpu":1}]}]`))
	f.Add([]byte(`[{"Events":[`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var rows Rows
		_ = rows.Unmarshal(data)
		rows.Reset()
	})
}

func FuzzStructuredEvents(f *testing.F) {
	f.Add([]byte("SystemSample"), uint8(0))
	f.Add([]byte("ProcessSample"), uint8(1))
	f.Add([]byte{}, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, count uint8) {
		if len(data) > 4096 {
			t.Skip()
		}
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		var events []map[string]any
		for i := 0; i < int(count%3)+1; i++ {
			var (
				eventType string
				timestamp int64
				tags      map[string]string
				samples   map[string]float64
			)
			r.Fill(&eventType)
			r.Fill(&timestamp)
			r.Fill(&tags)
			r.Fill(&samples)
			event := make(map[string]any, len(tags)+len(samples)+2)
			for key, value := range tags {
				event["tag_"+key] = value
			}
			for key, value := range samples {
				event["sample_"+key] = value
			}
			event["eventType"] = eventType
			event["timestamp"] = timestamp
			events = append(events, event)
		}
		b, err := json.Marshal([]map[string]any{{"Events": events}})
		if err != nil {
			return
		}
		var rows Rows
		_ = rows.Unmarshal(b)
		rows.Reset()
	})
}
