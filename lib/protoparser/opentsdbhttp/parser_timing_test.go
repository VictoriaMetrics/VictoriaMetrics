package opentsdbhttp

import (
	"fmt"
	"testing"

	"github.com/valyala/fastjson"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `[{"metric": "cpu.usage_user", "timestamp": 1234556768, "value": 1.23, "tags": {"a":"b", "x": "y"}},
{"metric": "cpu.usage_system", "timestamp": 1234556768, "value": 23.344, "tags": {"a":"b"}},
{"metric": "cpu.usage_iowait", "timestamp": 1234556769, "value":3.3443, "tags": {"a":"b"}},
{"metric": "cpu.usage_irq", "timestamp": 1234556768, "value": 0.34432, "tags": {"a":"b"}}
]
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		var p fastjson.Parser
		for pb.Next() {
			v, err := p.Parse(s)
			if err != nil {
				panic(fmt.Errorf("cannot parse %q: %w", s, err))
			}
			rows.Unmarshal(v)
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows unmarshaled; got %d; want 4", len(rows.Rows)))
			}
		}
	})
}
