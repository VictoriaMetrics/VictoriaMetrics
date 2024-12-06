package logstorage

import (
	"fmt"
	"testing"
)

func BenchmarkBlock_MustInitFromRows(b *testing.B) {
	for _, rowsPerBlock := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("rowsPerBlock_%d", rowsPerBlock), func(b *testing.B) {
			benchmarkBlockMustInitFromRows(b, rowsPerBlock)
		})
	}
}

func benchmarkBlockMustInitFromRows(b *testing.B, rowsPerBlock int) {
	timestamps, rows := newTestRows(rowsPerBlock, 10)
	b.ReportAllocs()
	b.SetBytes(int64(len(timestamps)))
	b.RunParallel(func(pb *testing.PB) {
		block := getBlock()
		defer putBlock(block)
		for pb.Next() {
			block.MustInitFromRows(timestamps, rows)
			if n := block.Len(); n != len(timestamps) {
				panic(fmt.Errorf("unexpected block length; got %d; want %d", n, len(timestamps)))
			}
		}
	})
}

func newTestRows(rowsCount, fieldsPerRow int) ([]int64, [][]Field) {
	timestamps := make([]int64, rowsCount)
	rows := make([][]Field, rowsCount)
	for i := range timestamps {
		timestamps[i] = int64(i) * 1e9
		fields := make([]Field, fieldsPerRow)
		for j := range fields {
			f := &fields[j]
			f.Name = fmt.Sprintf("field_%d", j)
			f.Value = fmt.Sprintf("value_%d_%d", i, j)
		}
		rows[i] = fields
	}
	return timestamps, rows
}
