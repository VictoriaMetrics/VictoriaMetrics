package prometheus

import (
	"encoding/hex"
	"strconv"
	"strings"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzUnmarshalWithMetadata(f *testing.F) {
	f.Add([]byte("# HELP requests_total Count\n# TYPE requests_total counter\nrequests_total{job=\"test\"} 1 42\n"))
	f.Add([]byte("metric 1\n"))
	f.Add([]byte("metric{x=\"unterminated"))
	f.Add([]byte("metric{host=\"a\\\"b\",region=\"us\"} 1 42\nmetric{host=\"c\"} 2\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		rows, metadata := UnmarshalWithMetadata(Rows{}, MetadataRows{}, string(data), func(string) {})
		rows.Reset()
		metadata.Reset()
		rows.UnmarshalWithErrLogger(string(data), func(string) {})
		rows.Reset()
	})
}

func FuzzStructuredRows(f *testing.F) {
	f.Add([]byte("metric"))
	f.Add([]byte{0, 1, 2, 3})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var row Row
		r := randfill.NewFromGoFuzz(data).NilChance(0).NumElements(0, 64).Funcs(func(tag *Tag, c randfill.Continue) {
			tag.Key = "label_" + hex.EncodeToString([]byte(c.String(65535)))
			tag.Value = "value_" + hex.EncodeToString([]byte(c.String(65535)))
		})
		r.Fill(&row)
		metric := "metric_" + hex.EncodeToString([]byte(row.Metric))
		labels := make([]string, 0, len(row.Tags))
		for _, tag := range row.Tags {
			labels = append(labels, tag.Key+"="+strconv.Quote(tag.Value))
		}
		sample := metric + "{" + strings.Join(labels, ",") + "} " + strconv.FormatFloat(row.Value, 'f', -1, 64) + "\n"
		input := "# HELP " + metric + " Generated metric\n# TYPE " + metric + " gauge\n" + sample

		rows, metadata := UnmarshalWithMetadata(Rows{}, MetadataRows{}, input, func(string) {})
		if len(rows.Rows) != 1 || rows.Rows[0].Metric != metric || len(rows.Rows[0].Tags) != len(labels) || rows.Rows[0].Value != row.Value {
			t.Fatalf("cannot parse generated Prometheus row: %q", input)
		}
		rows.Reset()
		metadata.Reset()
		rows.UnmarshalWithErrLogger(input, func(string) {})
		rows.Reset()
	})
}

func FuzzSeriesDiff(f *testing.F) {
	f.Add("cpu{host=\"vm\"} 1\nmem 2\n", "cpu{host=\"vm\"} 3\n")
	f.Add("cpu 1 42\nmem 2\n", "mem 3\n")
	f.Add("cpu{host=\"a\\\"b\"} 1\n", "")
	f.Add("cpu 1 42\n", "cpu 2 43\n")
	f.Add("cpu{host=\"a b\"} NaN 42\n", "cpu{host=\"a b\"} +Inf 43\n")
	f.Add("cpu 1 # help\n", "cpu 2 # other\n")
	f.Add("mem 1\ncpu 2\n", "cpu 3\nmem 4\n")
	f.Fuzz(func(t *testing.T, a, b string) {
		if len(a)+len(b) > 4096 {
			t.Skip()
		}
		_ = GetRowsDiff(a, b)
		_ = AreIdenticalSeriesFast(a, b)
	})
}
