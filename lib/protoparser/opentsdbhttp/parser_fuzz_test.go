package opentsdbhttp

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte(`{"metric":"cpu","timestamp":42,"value":1,"tags":{"host":"vm"}}`))
	f.Add([]byte(`[{"metric":"cpu","timestamp":"42","value":"1","tags":{"host":"vm"}},{"metric":"mem","value":2}]`))
	f.Add([]byte(`{"metric":"cpu","timestamp":true,"value":false,"tags":{"host":12}}`))
	f.Add([]byte(`[{"metric":"cpu","value":1},`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		p := GetJSONParser()
		defer PutJSONParser(p)
		var rows Rows
		if v, err := p.ParseBytes(data); err == nil {
			rows.Unmarshal(v)
			rows.Reset()
		}
	})
}

func FuzzStructuredRows(f *testing.F) {
	f.Add([]byte("cpu"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var (
			metric    string
			timestamp int64
			value     float64
			tags      map[string]string
		)
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		r.Fill(&metric)
		r.Fill(&timestamp)
		r.Fill(&value)
		r.Fill(&tags)
		b, err := json.Marshal(map[string]any{
			"metric": metric, "timestamp": timestamp, "value": value, "tags": tags,
		})
		if err != nil {
			return
		}
		p := GetJSONParser()
		defer PutJSONParser(p)
		var rows Rows
		v, err := p.ParseBytes(b)
		if err != nil {
			return
		}
		rows.Unmarshal(v)
		rows.Reset()
	})
}
