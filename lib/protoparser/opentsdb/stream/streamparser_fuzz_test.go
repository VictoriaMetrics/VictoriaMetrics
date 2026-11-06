package stream

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("put cpu 42 1 host=vm\n"))
	f.Add([]byte("put cpu invalid 1 host=vm\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), func([]opentsdb.Row) error { return nil })
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
		var row opentsdb.Row
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		line := "put fuzz_" + hex.EncodeToString([]byte(row.Metric)) + " " + strconv.FormatInt(row.Timestamp, 10) + " 1"
		for _, tag := range row.Tags {
			line += " label_" + hex.EncodeToString([]byte(tag.Key)) + "=value_" + hex.EncodeToString([]byte(tag.Value))
		}
		line += " host=vm\n"
		var count int
		err := Parse(bytes.NewBufferString(line), func(rows []opentsdb.Row) error {
			count += len(rows)
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("cannot parse structured OpenTSDB row: count=%d, err=%v", count, err)
		}
	})
}
