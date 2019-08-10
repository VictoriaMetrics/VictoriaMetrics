package opentsdbhttp

import (
	"fmt"
	"testing"
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
		for pb.Next() {
			p := parserPool.Get()
			v, err := p.Parse(s)
			if err != nil {
				parserPool.Put(p)
				panic(fmt.Errorf("cannot parse %q: %s", s, err))
			}
			if err := rows.Unmarshal(v); err != nil {
				parserPool.Put(p)
				panic(fmt.Errorf("cannot unmarshal %q: %s", s, err))
			}
			parserPool.Put(p)
		}
	})
}
