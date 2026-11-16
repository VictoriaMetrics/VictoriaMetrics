package stream

import (
	"bytes"
	"encoding/csv"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/csvimport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"sigs.k8s.io/randfill"
)

const csvFormat = "1:metric:cpu,2:time:unix_ms,3:label:host"

func FuzzParse(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("1,42,vm\n"), csvFormat)
	f.Add([]byte("timestamp,cpu\n42,1\n"), "1:time:unix_s,2:metric:cpu")
	f.Add([]byte("\"unterminated"), "invalid")
	f.Fuzz(func(t *testing.T, data []byte, format string) {
		if len(data)+len(format) > 4096 {
			t.Skip()
		}
		req, err := http.NewRequest(http.MethodPost, "/api/v1/import/csv", bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		q := req.URL.Query()
		q.Set("format", format)
		req.URL.RawQuery = q.Encode()
		_ = Parse(req, func([]csvimport.Row) error { return nil })
	})
}

func FuzzUnmarshal(f *testing.F) {
	f.Add("1,42,vm\n", csvFormat)
	f.Add("value,time,host\n1,42,vm\n", csvFormat)
	f.Add("\"unterminated", "invalid")
	f.Fuzz(func(t *testing.T, data, format string) {
		if len(data)+len(format) > 4096 {
			t.Skip()
		}
		cds, err := csvimport.ParseColumnDescriptors(format)
		if err != nil {
			return
		}
		var rows csvimport.Rows
		rows.UnmarshalDetectHeader(data, cds)
		rows.Reset()
		rows.Unmarshal(data, cds)
		rows.Reset()
	})
}

func FuzzStructuredRows(f *testing.F) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f.Add([]byte("vm"), false)
	f.Add([]byte("host"), true)
	f.Fuzz(func(t *testing.T, data []byte, header bool) {
		if len(data) > 4096 {
			t.Skip()
		}
		var row struct {
			Value     float64
			Timestamp int64
			Host      *string
		}
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&row)
		if math.IsNaN(row.Value) || math.IsInf(row.Value, 0) {
			return
		}
		host := "host_"
		if row.Host != nil {
			host += strings.ReplaceAll(strings.ReplaceAll(*row.Host, "\n", "_"), "\r", "_")
		}
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		if header {
			_ = w.Write([]string{"cpu", "timestamp", "host"})
		}
		_ = w.Write([]string{strconv.FormatFloat(row.Value, 'g', -1, 64), strconv.FormatInt(row.Timestamp, 10), host})
		w.Flush()
		payload := buf.String()

		req, err := http.NewRequest(http.MethodPost, "/api/v1/import/csv", &buf)
		if err != nil {
			t.Fatal(err)
		}
		q := req.URL.Query()
		q.Set("format", csvFormat)
		req.URL.RawQuery = q.Encode()
		count := 0
		err = Parse(req, func(rows []csvimport.Row) error {
			count += len(rows)
			return nil
		})
		if err != nil || count != 1 {
			t.Fatalf("cannot parse structured CSV row %q: count=%d, err=%v", payload, count, err)
		}
	})
}
