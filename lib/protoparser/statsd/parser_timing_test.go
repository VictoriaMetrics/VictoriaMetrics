package statsd

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu.usage_user:1.23|c
cpu.usage_system:23.344|c
cpu.usage_iowait:3.3443|c
cpu.usage_irq:0.34432|c
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			rows.Unmarshal(s)
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows unmarshaled: got %d; want 4", len(rows.Rows)))
			}
		}
	})
}
