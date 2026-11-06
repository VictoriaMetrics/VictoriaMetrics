package stream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/zabbixconnector"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte(`{"host":{"host":"vm","name":"node"},"name":"cpu","value":1,"clock":42,"ns":0,"type":0,"item_tags":[]}` + "\n"))
	f.Add([]byte(`{"host":null,"value":"bad"}` + "\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]zabbixconnector.Row) error { return nil })
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
		var row struct {
			Host struct {
				Host string `json:"host"`
				Name string `json:"name"`
			} `json:"host"`
			Name     string              `json:"name"`
			Value    float64             `json:"value"`
			Clock    int64               `json:"clock"`
			NS       int64               `json:"ns"`
			Type     int64               `json:"type"`
			Groups   []string            `json:"groups"`
			ItemTags []map[string]string `json:"item_tags"`
		}
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		row.Host.Host = "host_" + row.Host.Host
		row.Host.Name = "node_" + row.Host.Name
		row.Name = "metric_" + row.Name
		row.Type = 0
		row.ItemTags = append(row.ItemTags, map[string]string{"tag": "fuzz", "value": "1"})
		body, err := json.Marshal(row)
		if err != nil {
			return
		}
		var got int
		err = Parse(bytes.NewReader(append(body, '\n')), "", func(rows []zabbixconnector.Row) error {
			got += len(rows)
			return nil
		})
		if err != nil || got != 1 {
			t.Fatalf("cannot parse structured Zabbix row: count=%d, err=%v", got, err)
		}
	})
}
