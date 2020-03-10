package csvimport

import (
	"fmt"
	"testing"
)

func BenchmarkRowsUnmarshal(b *testing.B) {
	cds, err := ParseColumnDescriptors("1:label:symbol,2:metric:bid,3:metric:ask,4:time:unix_ms")
	if err != nil {
		b.Fatalf("cannot parse column descriptors: %s", err)
	}
	s := `GOOG,123.456,789.234,1345678999003
GOOG,223.456,889.234,1345678939003
GOOG,323.456,989.234,1345678949003
MSFT,423.456,189.234,1345678959003
AMZN,523.456,189.234,1345678959005
`
	const rowsExpected = 10
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var rs Rows
		for pb.Next() {
			rs.Unmarshal(s, cds)
			if len(rs.Rows) != rowsExpected {
				panic(fmt.Errorf("unexpected rows parsed; got %d; want %d; rows: %v", len(rs.Rows), rowsExpected, rs.Rows))
			}
		}
	})
}
