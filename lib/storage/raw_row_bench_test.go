package storage

import (
	"testing"
	"unsafe"
)

func BenchmarkRawRowsShardAddRows(b *testing.B) {
	const batchRows = 1024
	for _, tc := range []struct {
		name  string
		rows  int
		batch int
	}{
		{name: "sparse", rows: 128, batch: 128},
		{name: "growing", rows: 3 * batchRows, batch: batchRows},
		{name: "full", rows: maxRawRowsPerShard, batch: maxRawRowsPerShard},
	} {
		src := make([]rawRow, tc.rows)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(src)) * int64(unsafe.Sizeof(rawRow{})))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var shard rawRowsShard
				for rows := src; len(rows) > 0; {
					n := min(len(rows), tc.batch)
					shard.addRows(rows[:n])
					rows = rows[n:]
				}
				if i == 0 {
					b.ReportMetric(float64(cap(shard.rows)*int(unsafe.Sizeof(rawRow{}))), "shard-bytes")
				}
			}
		})
	}
}
