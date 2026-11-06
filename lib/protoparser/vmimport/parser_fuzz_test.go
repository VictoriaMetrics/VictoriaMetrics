package vmimport

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte(`{"metric":{"__name__":"test"},"values":[1],"timestamps":[42]}`))
	f.Add([]byte(`{"metric":{"__name__":"test","host":"vm"},"values":[null,"Inf","-Infinity",1],"timestamps":[1,2,3,4]}`))
	f.Add([]byte(`{"metric":{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var rows Rows
		rows.Unmarshal(string(data))
		rows.Reset()
	})
}

func FuzzStructuredRows(f *testing.F) {
	f.Add([]byte("test"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var (
			metric     map[string]string
			values     []float64
			timestamps []int64
		)
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		r.Fill(&metric)
		r.Fill(&values)
		r.Fill(&timestamps)
		b, err := json.Marshal(map[string]any{
			"metric": metric, "values": values, "timestamps": timestamps,
		})
		if err != nil {
			return
		}
		var rows Rows
		rows.Unmarshal(string(b))
		rows.Reset()
	})
}
