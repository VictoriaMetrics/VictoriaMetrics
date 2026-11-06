package zabbixconnector

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte(`{"host":{"host":"vm","name":"node"},"name":"cpu","value":1,"clock":42,"ns":0,"type":0,"item_tags":[]}` + "\n"))
	f.Add([]byte(`{"host":{"host":"vm","name":"node"},"name":"cpu","value":1,"clock":42,"ns":0,"type":3,"item_tags":"bad"}` + "\n"))
	f.Add([]byte(`{"type":"bad"}` + "\n"))
	f.Add([]byte(`{"host":null,"value":"bad"}` + "\n"))
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
	// Enable host groups, empty tags, and set duplicate tag separator to comma
	*addGroupsValue, *addEmptyTagsValue, *addDuplicateTagsSeparator = "1", "1", ","

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
		row.Groups = append(row.Groups, "fuzz")
		row.ItemTags = append(row.ItemTags,
			map[string]string{"tag": "fuzz", "value": string(data)},
			map[string]string{"tag": "fuzz", "value": ""})
		b, err := json.Marshal(row)
		if err != nil {
			return
		}
		var rows Rows
		rows.Unmarshal(string(b) + "\n")
		if len(rows.Rows) != 1 {
			t.Fatalf("expected one parsed row for %s", b)
		}
		rows.Reset()
	})
}
