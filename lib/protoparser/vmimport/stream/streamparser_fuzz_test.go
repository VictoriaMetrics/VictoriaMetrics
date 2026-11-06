package stream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/vmimport"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte(`{"metric":{"__name__":"cpu"},"values":[1],"timestamps":[42]}` + "\n"))
	f.Add([]byte(`{"metric":` + "\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]vmimport.Row) error { return nil })
	})
}

func FuzzStructuredRows(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var (
			metric    map[string]string
			values    []float64
			timestamp int64
		)
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		r.Fill(&metric)
		r.Fill(&values)
		if metric == nil {
			metric = make(map[string]string)
		}
		metric["__name__"] = "fuzz_metric"
		values = append(values, 1)
		timestamps := make([]int64, len(values))
		for i := range timestamps {
			r.Fill(&timestamp)
			timestamps[i] = timestamp
		}
		body, err := json.Marshal(map[string]any{
			"metric": metric, "values": values, "timestamps": timestamps,
		})
		if err != nil {
			return
		}
		var got int
		err = Parse(bytes.NewReader(append(body, '\n')), "", func(rows []vmimport.Row) error {
			got += len(rows)
			return nil
		})
		if err != nil || got != 1 {
			t.Fatalf("cannot parse structured VM import row: count=%d, err=%v", got, err)
		}
	})
}
