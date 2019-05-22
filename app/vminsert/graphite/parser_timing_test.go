package graphite

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu.usage_user 1.23 1234556768
cpu.usage_system 23.344 1234556768
cpu.usage_iowait 3.3443 1234556769
cpu.usage_irq 0.34432 1234556768
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			if err := rows.Unmarshal(s); err != nil {
				panic(fmt.Errorf("cannot unmarshal %q: %s", s, err))
			}
		}
	})
}
