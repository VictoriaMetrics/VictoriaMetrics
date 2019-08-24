package influx

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	s := `cpu usage_user=1.23,usage_system=4.34,usage_iowait=0.1112 1234556768
cpu usage_user=1.23,usage_system=4.34,usage_iowait=0.1112 123455676344
aaa usage_user=1.23,usage_system=4.34,usage_iowait=0.1112 123455676344
bbb usage_user=1.23,usage_system=4.34,usage_iowait=0.1112 123455676344
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			rows.Unmarshal(s)
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows parsed; got %d; want 4", len(rows.Rows)))
			}
		}
	})
}
