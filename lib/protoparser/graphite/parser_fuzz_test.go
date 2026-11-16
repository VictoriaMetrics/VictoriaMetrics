package graphite

import (
	"encoding/hex"
	"strconv"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte("cpu.load 1 42\n"))
	f.Add([]byte("cpu.load;host=vm 1.5 42\n"))
	f.Add([]byte("cpu.load invalid timestamp\n"))
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
	f.Add([]byte("cpu"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var row Row
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		line := "fuzz_" + hex.EncodeToString([]byte(row.Metric))
		for _, tag := range row.Tags {
			line += ";label_" + hex.EncodeToString([]byte(tag.Key)) + "=value_" + hex.EncodeToString([]byte(tag.Value))
		}
		line += " " + strconv.FormatFloat(row.Value, 'g', -1, 64) + " " + strconv.FormatInt(row.Timestamp, 10) + "\n"
		var rows Rows
		rows.Unmarshal(line)
		rows.Reset()
	})
}
