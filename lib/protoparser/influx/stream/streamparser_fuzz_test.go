package stream

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu,host=vm usage=1.5 42\n"), "ns")
	f.Add([]byte("cpu usage=1i 42\ninvalid\n"), "ms")
	f.Add([]byte("cpu usage=1 0\n"), "")
	f.Add([]byte("cpu usage=1 100000000000\n"), "")
	f.Add([]byte("cpu usage=1 100000000000000\n"), "")
	f.Add([]byte("cpu usage=1 100000000000000000\n"), "")
	f.Add([]byte("cpu usage=1 42\n"), "us")
	f.Add([]byte("cpu usage=1 42\n"), "h")
	f.Fuzz(func(t *testing.T, data []byte, precision string) {
		if len(data)+len(precision) > 4096 {
			t.Skip()
		}
		for _, streamMode := range []bool{false, true} {
			_ = Parse(bytes.NewReader(data), "", streamMode, precision, "fuzz", func(string, []influx.Row) error { return nil })
		}
	})
}

func FuzzStructuredRows(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu"), false)
	f.Add([]byte{}, true)
	f.Fuzz(func(t *testing.T, data []byte, streamMode bool) {
		if len(data) > 4096 {
			t.Skip()
		}
		var row influx.Row
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		line := "fuzz_" + hex.EncodeToString([]byte(row.Measurement))
		for _, tag := range row.Tags {
			line += ",tag_" + hex.EncodeToString([]byte(tag.Key)) + "=value_" + hex.EncodeToString([]byte(tag.Value))
		}
		line += " value=1i " + strconv.FormatInt(row.Timestamp, 10) + "\n"
		var count int
		err := Parse(bytes.NewBufferString(line), "", streamMode, "ms", "fuzz", func(db string, rows []influx.Row) error {
			count += len(rows)
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("cannot parse structured Influx row: count=%d, err=%v", count, err)
		}
	})
}
