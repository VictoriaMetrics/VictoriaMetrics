package csvimport

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte("1,42,vm\n"))
	f.Add([]byte("\"unterminated,42"))
	f.Add([]byte("1,42,\"a,b\"\n2,43,\"a\"\"b\"\n"))
	cds, err := ParseColumnDescriptors("1:metric:cpu,2:time:unix_s,3:label:host")
	if err != nil {
		f.Fatal(err)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var rows Rows
		rows.Unmarshal(string(data), cds)
		rows.Reset()
	})
}

func FuzzStructuredRows(f *testing.F) {
	f.Add([]byte("vm"))
	f.Add([]byte{})
	cds, err := ParseColumnDescriptors("1:metric:cpu,2:time:unix_s,3:label:host")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var record struct {
			Value     float64
			Timestamp int64
			Host      *string
		}
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&record)
		var host string
		if record.Host != nil {
			host = *record.Host
		}
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{
			strconv.FormatFloat(record.Value, 'g', -1, 64),
			strconv.FormatInt(record.Timestamp, 10), host,
		})
		w.Flush()
		var rows Rows
		rows.Unmarshal(buf.String(), cds)
		rows.Reset()
	})
}

func FuzzColumnDescriptors(f *testing.F) {
	f.Add("1:metric:cpu,2:time:unix_ms,3:label:host", "value,time,host\n1,42,vm\n")
	f.Add("1:metric:cpu,2:time:unix_ns", "1,42000000\n")
	f.Add("1:metric:cpu,2:time:rfc3339", "1,2024-01-02T03:04:05Z\n")
	f.Add("1:metric:cpu,2:time:custom:2006-01-02", "1,2024-01-02\n")
	f.Add("1:metric:cpu,2:time:unix_s", "1,42\n")
	f.Fuzz(func(t *testing.T, descriptor, data string) {
		if len(descriptor) > 1024 || len(data) > 4096 {
			t.Skip()
		}
		cds, err := ParseColumnDescriptors(descriptor)
		if err != nil {
			return
		}
		var rows Rows
		rows.UnmarshalDetectHeader(data, cds)
		rows.Reset()
	})
}
