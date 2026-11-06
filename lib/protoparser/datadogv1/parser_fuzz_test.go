package datadogv1

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRequestUnmarshal(f *testing.F) {
	f.Add([]byte(`{"series":[{"metric":"cpu","points":[[42,1]],"tags":["host:vm"]}]}`))
	f.Add([]byte(`{"series":[{"points":[`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var req Request
		_ = req.Unmarshal(data)

		var generated Request
		randfill.NewFromGoFuzz(data).NumElements(0, 3).Fill(&generated)
		if b, err := json.Marshal(generated); err == nil {
			_ = req.Unmarshal(b)
		}
	})
}
