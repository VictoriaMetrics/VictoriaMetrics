package prometheus

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu_usage{mode="user"} 1.23
cpu_usage{mode="system"} 23.344
cpu_usage{mode="iowait"} 3.3443
cpu_usage{mode="irq"} 0.34432
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
