package stream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`[{"Events":[{"eventType":"SystemSample","timestamp":42,"cpu":1}]}]`))
	f.Add([]byte(`[{"Events":[`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]newrelic.Row) error { return nil })
	})
}

func FuzzStructuredEvents(f *testing.F) {
	f.Add([]byte("SystemSample"), uint8(0))
	f.Add([]byte{}, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, count uint8) {
		if len(data) > 4096 {
			t.Skip()
		}
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		events := make([]map[string]any, 0, int(count%3)+1)
		for i := 0; i < cap(events); i++ {
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
			event := map[string]any{"eventType": "fuzz_" + eventType, "timestamp": timestamp, "value": 1}
			for k, v := range tags {
				event["tag_"+k] = v
			}
			for k, v := range samples {
				event["sample_"+k] = v
			}
			events = append(events, event)
		}
		body, err := json.Marshal([]map[string]any{{"Events": events}})
		if err != nil {
			return
		}
		var got int
		err = Parse(bytes.NewReader(body), "", func(rows []newrelic.Row) error {
			got += len(rows)
			return nil
		})
		if err != nil || got != len(events) {
			t.Fatalf("cannot parse structured New Relic events: count=%d, err=%v", got, err)
		}
	})
}
