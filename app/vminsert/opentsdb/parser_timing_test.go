package opentsdb

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `put cpu.usage_user 1234556768 1.23 a=b
put cpu.usage_system 1234556768 23.344 a=b
put cpu.usage_iowait 1234556769 3.3443 a=b
put cpu.usage_irq 1234556768 0.34432 a=b
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
