package influx

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu usage_user=1.23,usage_system=4.34,usage_iowait=0.1112 1234556768`
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
