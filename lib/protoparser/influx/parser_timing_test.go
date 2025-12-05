package influx

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshalNoEscaping(b *testing.B) {
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
			if err := rows.Unmarshal(s, false); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows parsed; got %d; want 4", len(rows.Rows)))
			}
		}
	})
}

func BenchmarkRowsUnmarshalWithEscaping(b *testing.B) {
	s := `cpu usage\ user=1.23,usage\ system=4.34,usage\,iowait=0.1112 1234556768
cpu usage\ user=1.23,usage\ system=4.34,usage\,iowait=0.1112 123455676344
aaa usage\ user=1.23,usage\ system=4.34,usage\,iowait=0.1112 123455676344
bbb usage\ user=1.23,usage\ system=4.34,usage\,iowait=0.1112 123455676344
`
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rows Rows
		for pb.Next() {
			if err := rows.Unmarshal(s, false); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(rows.Rows) != 4 {
				panic(fmt.Errorf("unexpected number of rows parsed; got %d; want 4", len(rows.Rows)))
			}
		}
	})
}
