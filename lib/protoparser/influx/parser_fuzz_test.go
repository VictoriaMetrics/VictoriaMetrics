package influx

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzRowsUnmarshal(f *testing.F) {
	f.Add([]byte("cpu,host=vm value=1 42\n"))
	f.Add([]byte("cpu,host=\\,x value=\"unterminated"))
	f.Add([]byte("cpu,host=vm,env=prod value=2i,ok=t,limit=3u 42\n"))
	f.Add([]byte("cpu,host=\\,x value=\"1,2\",other=3 42\n"))
	f.Add([]byte("cpu,host value=1 42\n"))
	f.Add([]byte("cpu value=1i,missing 42\n"))
	f.Add([]byte("# comment\ncpu value=3\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var rows Rows
		_ = rows.Unmarshal(string(data), false)
		rows.Reset()
		_ = rows.Unmarshal(string(data), true)
		rows.Reset()
	})
}

func FuzzStructuredRows(f *testing.F) {
	f.Add([]byte("cpu"))
	f.Add([]byte{})
	escape := strings.NewReplacer(`\`, `\\`, ",", `\,`, " ", `\ `, "=", `\=`, "\n", "_", "\r", "_")
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var row Row
		var text string
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		r.Fill(&row)
		r.Fill(&text)
		var b strings.Builder
		b.WriteString("fuzz_" + escape.Replace(row.Measurement))
		for _, tag := range row.Tags {
			b.WriteString(",tag_" + escape.Replace(tag.Key) + "=" + escape.Replace(tag.Value))
		}
		b.WriteString(" fuzz_value=1,fuzz_text=" + strconv.Quote(text))
		for _, field := range row.Fields {
			if math.IsNaN(field.Value) || math.IsInf(field.Value, 0) {
				continue
			}
			key := strings.ReplaceAll(field.Key, `"`, "_")
			b.WriteString(",field_" + escape.Replace(key) + "=" + strconv.FormatFloat(field.Value, 'g', -1, 64))
		}
		b.WriteString(" " + strconv.FormatInt(row.Timestamp, 10) + "\n")
		var rows Rows
		if err := rows.Unmarshal(b.String(), false); err != nil || len(rows.Rows) != 1 {
			t.Fatalf("cannot parse structured Influx row %q: count=%d, err=%v", b.String(), len(rows.Rows), err)
		}
		rows.Reset()
	})
}
