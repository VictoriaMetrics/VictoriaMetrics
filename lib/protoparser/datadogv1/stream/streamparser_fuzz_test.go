package stream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv1"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"series":[{"metric":"cpu","points":[[42,1]]}]}`))
	f.Add([]byte(`{"series":[`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]datadogv1.Series) error { return nil })
	})
}

func FuzzStructuredRequest(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("series with tags and points"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var req datadogv1.Request
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&req)
		// Ensure at least one series is present
		req.Series = append(req.Series, datadogv1.Series{
			Metric: "cpu",
			Points: []datadogv1.Point{{42, 1}},
		})

		body, err := json.Marshal(req)
		if err != nil {
			t.Skip()
		}

		called := false
		err = Parse(bytes.NewReader(body), "", func(series []datadogv1.Series) error {
			called = true
			if len(series) != len(req.Series) {
				t.Fatalf("got %d series; want %d", len(series), len(req.Series))
			}
			return nil
		})
		if err != nil || !called {
			t.Fatalf("valid request: err=%v, callback called=%v", err, called)
		}
	})
}
