package stream

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/graphite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu.load 1 42\n"))
	f.Add([]byte("cpu.load;host=vm 1.5 42\ninvalid\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]graphite.Row) error { return nil })
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
		var row graphite.Row
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		line := "fuzz_" + hex.EncodeToString([]byte(row.Metric))
		for _, tag := range row.Tags {
			line += ";label_" + hex.EncodeToString([]byte(tag.Key)) + "=value_" + hex.EncodeToString([]byte(tag.Value))
		}
		line += " 1 " + strconv.FormatInt(row.Timestamp, 10) + "\n"
		var count int
		err := Parse(bytes.NewBufferString(line), "", func(rows []graphite.Row) error {
			count += len(rows)
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("cannot parse structured Graphite row: count=%d, err=%v", count, err)
		}
	})
}
