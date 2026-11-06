package stream

import (
	"bytes"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("# HELP cpu usage\n# TYPE cpu gauge\ncpu{host=\"vm\"} 1 42\n"))
	f.Add([]byte("cpu{host=\"unterminated\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		for _, metadata := range []bool{false, true} {
			_ = Parse(bytes.NewReader(data), 42, "", false, metadata,
				func([]prometheus.Row, []prometheus.Metadata) error { return nil }, func(string) {})
		}
	})
}

func FuzzStructuredRows(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("cpu"), false)
	f.Add([]byte{}, true)
	f.Fuzz(func(t *testing.T, data []byte, metadata bool) {
		if len(data) > 4096 {
			t.Skip()
		}
		var row prometheus.Row
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		metric := "metric_" + hex.EncodeToString([]byte(row.Metric))
		labels := make([]string, 0, len(row.Tags))
		for _, tag := range row.Tags {
			labels = append(labels, "label_"+hex.EncodeToString([]byte(tag.Key))+"="+strconv.Quote(tag.Value))
		}
		input := "# HELP " + metric + " Generated metric\n# TYPE " + metric + " gauge\n" +
			metric + "{" + strings.Join(labels, ",") + "} 1 42\n"
		var gotRows, gotMetadata int
		err := Parse(bytes.NewBufferString(input), 42, "", false, metadata,
			func(rows []prometheus.Row, m []prometheus.Metadata) error {
				gotRows += len(rows)
				gotMetadata += len(m)
				return nil
			}, func(string) {})
		if err != nil || gotRows != 1 || (metadata && gotMetadata != 1) {
			t.Fatalf("cannot parse structured Prometheus row: rows=%d, metadata=%d, err=%v", gotRows, gotMetadata, err)
		}
	})
}
