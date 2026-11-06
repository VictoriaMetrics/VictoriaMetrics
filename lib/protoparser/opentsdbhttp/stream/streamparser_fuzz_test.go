package stream

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdbhttp"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"metric":"cpu","timestamp":42,"value":1,"tags":{"host":"vm"}}`))
	f.Add([]byte(`{"metric":`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		req, err := http.NewRequest(http.MethodPost, "/api/put", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		_ = Parse(req, func([]opentsdbhttp.Row) error { return nil })
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
		if tags == nil {
			tags = map[string]string{}
		}
		body, err := json.Marshal(map[string]any{
			"metric": "fuzz_" + metric, "timestamp": timestamp, "value": value, "tags": tags,
		})
		if err != nil {
			return
		}
		req, err := http.NewRequest(http.MethodPost, "/api/put", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var got int
		err = Parse(req, func(rows []opentsdbhttp.Row) error {
			got += len(rows)
			return nil
		})
		if err != nil || got != 1 {
			t.Fatalf("cannot parse structured OpenTSDB HTTP row: count=%d, err=%v", got, err)
		}
	})
}
