package opentsdb

import (
	"encoding/hex"
	"strconv"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte("put cpu 42 1 host=vm\n"))
	f.Add([]byte("put cpu 42 1\nput mem 43 -2 host=vm\r\n"))
	f.Add([]byte("put cpu not-a-timestamp 1 host=vm\n"))
	f.Add([]byte("put cpu 42 1 host\n"))
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
		line := "put fuzz_" + hex.EncodeToString([]byte(row.Metric)) + " " +
			strconv.FormatInt(row.Timestamp, 10) + " " + strconv.FormatFloat(row.Value, 'g', -1, 64)
		for _, tag := range row.Tags {
			line += " label_" + hex.EncodeToString([]byte(tag.Key)) + "=value_" + hex.EncodeToString([]byte(tag.Value))
		}
		var rows Rows
		rows.Unmarshal(line)
		rows.Reset()
	})
}
