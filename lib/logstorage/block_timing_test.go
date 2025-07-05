package logstorage

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"unsafe"
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
	b.ResetTimer()
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

type blockTestData struct {
	timestamps []int64
	rows       [][]Field
	b          []byte // arena
}

const bytesPerCol int = 50

func (td *blockTestData) generateColName(colIndex int) string {
	ptr := unsafe.Pointer(unsafe.SliceData(td.b))
	start := len(td.b)
	td.b = fmt.Appendf(td.b, "col_%05d", colIndex)
	end := len(td.b)
	colName := unsafe.String((*byte)(unsafe.Add(ptr, start)), end-start)
	return colName
}

func (td *blockTestData) copyString(s string) string {
	ptr := unsafe.Pointer(unsafe.SliceData(td.b))
	start := len(td.b)
	td.b = append(td.b, s...)
	end := len(td.b)
	colName := unsafe.String((*byte)(unsafe.Add(ptr, start)), end-start)
	return colName
}

func (td *blockTestData) generateColNames(cnt int) []string {
	colNames := make([]string, 0, cnt)
	for i := 0; i < cnt; i++ {
		s := fmt.Sprintf("col_%d", i+200+rand.IntN(1000000))
		colNames = append(colNames, s)
	}
	return colNames
}

func (td *blockTestData) generateColumnValue(length int) string {
	ptr := unsafe.Pointer(unsafe.SliceData(td.b))
	start := len(td.b)
	for i := 0; i < length; i++ {
		td.b = append(td.b, byte(rand.IntN(126-32)+32))
	}
	end := len(td.b)
	colValue := unsafe.String((*byte)(unsafe.Add(ptr, start)), end-start)
	return colValue
}

func (td *blockTestData) generateConstColValue(s string) string {
	ptr := unsafe.Pointer(unsafe.SliceData(td.b))
	start := len(td.b)
	td.b = append(td.b, s...)
	end := len(td.b)
	colValue := unsafe.String((*byte)(unsafe.Add(ptr, start)), end-start)
	return colValue
}

func (td *blockTestData) generateTimestamps(rowsCount int) {
	td.timestamps = make([]int64, rowsCount)
	for i := 0; i < rowsCount; i++ {
		td.timestamps[i] = int64(i) + 1e9
	}
}

func (td *blockTestData) GenerateTableLikeData(rowsCount int, colCount int) {
	td.generateTimestamps(rowsCount)
	td.b = make([]byte, 0, colCount*bytesPerCol*rowsCount*2)
	td.rows = make([][]Field, 0, rowsCount)
	for i := 0; i < rowsCount; i++ {
		row := make([]Field, 0, colCount)
		for j := 0; j < colCount; j++ {
			if cap(td.b)-len(td.b) < bytesPerCol {
				goto endGen
			}
			row = append(row, Field{td.generateColName(j), td.generateColumnValue(bytesPerCol - 9 - 3)})
		}
		row = append(row, Field{td.generateColName(colCount + 1), td.generateConstColValue("this is const column")})
		rand.Shuffle(len(row), func(i, j int) {
			row[i], row[j] = row[j], row[i]
		})
		td.rows = append(td.rows, row)
	}
endGen:
	if len(td.rows) != len(td.timestamps) {
		panic("not same")
	}
}

func (td *blockTestData) GenerateTableLikeDataWithDiffValueLen(rowsCount int, colCount int) {
	td.generateTimestamps(rowsCount)
	td.b = make([]byte, 0, colCount*bytesPerCol*rowsCount*2)
	td.rows = make([][]Field, 0, rowsCount)
	for i := 0; i < rowsCount; i++ {
		row := make([]Field, 0, colCount)
		for j := 0; j < colCount; j++ {
			if cap(td.b)-len(td.b) < bytesPerCol {
				goto endGen2
			}
			row = append(row, Field{td.generateColName(j), td.generateColumnValue(rand.IntN(bytesPerCol - 12))})
		}
		row = append(row, Field{td.generateColName(colCount + 1), td.generateConstColValue("this is const column")})
		rand.Shuffle(len(row), func(i, j int) {
			row[i], row[j] = row[j], row[i]
		})
		td.rows = append(td.rows, row)
	}
endGen2:
	if len(td.rows) != len(td.timestamps) {
		panic("not same")
	}
}

func (td *blockTestData) GenerateDiffNameCols(rowsCount int, colCount int) {
	td.generateTimestamps(rowsCount)
	td.b = make([]byte, 0, colCount*bytesPerCol*rowsCount*2)
	td.rows = make([][]Field, 0, rowsCount)
	colNames := td.generateColNames(1500)
	for i := 0; i < rowsCount; i++ {
		row := make([]Field, 0, colCount)
		for j := 0; j < colCount; j++ {
			if cap(td.b)-len(td.b) < bytesPerCol {
				goto endGenerateDiffNameCols
			}
			colName := colNames[rand.IntN(len(colNames))]
			row = append(row, Field{td.copyString(colName), td.generateColumnValue(rand.IntN(bytesPerCol - 12))})
		}
		row = append(row, Field{td.generateColName(colCount + 1), td.generateConstColValue("this is const column")})
		rand.Shuffle(len(row), func(i, j int) {
			row[i], row[j] = row[j], row[i]
		})
		td.rows = append(td.rows, row)
	}
endGenerateDiffNameCols:
	if len(td.rows) != len(td.timestamps) {
		panic("not same")
	}
}

func (td *blockTestData) GenerateRandomColCount(rowsCount int, colCount int) {
	td.generateTimestamps(rowsCount)
	td.b = make([]byte, 0, colCount*bytesPerCol*rowsCount*2)
	td.rows = make([][]Field, 0, rowsCount)
	for i := 0; i < rowsCount; i++ {
		row := make([]Field, 0, colCount)
		newColCount := rand.IntN(colCount-1) + 1
		for j := 0; j < newColCount; j++ {
			if cap(td.b)-len(td.b) < bytesPerCol {
				goto endGenRandomColCount
			}
			row = append(row, Field{td.generateColName(j), td.generateColumnValue(rand.IntN(bytesPerCol - 12))})
		}
		row = append(row, Field{td.generateColName(colCount + 1), td.generateConstColValue("this is const column")})
		rand.Shuffle(len(row), func(i, j int) {
			row[i], row[j] = row[j], row[i]
		})
		td.rows = append(td.rows, row)
	}
endGenRandomColCount:
	if len(td.rows) != len(td.timestamps) {
		panic("not same")
	}
}

func (td *blockTestData) BytesSize() int {
	return len(td.timestamps)*8 + len(td.b)
}

func runBench(td *blockTestData, b *testing.B) {
	block := getBlock()
	defer putBlock(block)
	b.ReportAllocs()
	b.SetBytes(int64(td.BytesSize()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block.MustInitFromRows(td.timestamps, td.rows)
		if n := block.Len(); n != len(td.timestamps) {
			panic(fmt.Errorf("unexpected block length; got %d; want %d", n, len(td.timestamps)))
		}
	}
}

/*
GOMAXPROCS=1 go test -benchmem -run=^$ -bench ^BenchmarkBlock_MustInitFromRowsV2$ github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
*/
func BenchmarkBlock_MustInitFromRowsV2(b *testing.B) {
	b.Run("same_column_count", func(b *testing.B) {
		td := &blockTestData{}
		td.GenerateTableLikeData(1000, 100) // 1000 rows，100 columns
		runBench(td, b)
	})
	b.Run("same_column_count_random_value_len", func(b *testing.B) {
		td := &blockTestData{}
		td.GenerateTableLikeDataWithDiffValueLen(1000, 100)
		runBench(td, b)
	})
	b.Run("random_column_count", func(b *testing.B) {
		td := &blockTestData{}
		td.GenerateRandomColCount(1000, 100)
		runBench(td, b)
	})
	b.Run("diff_col_name", func(b *testing.B) {
		td := &blockTestData{}
		td.GenerateDiffNameCols(1000, 100)
		runBench(td, b)
	})
}

func Test_block_v2(t *testing.T) {
	f := func(td *blockTestData) {
		block := getBlock()
		defer putBlock(block)
		block.MustInitFromRows(td.timestamps, td.rows)
		if n := block.Len(); n != len(td.timestamps) {
			t.Errorf("unexpected block length; got %d; want %d", n, len(td.timestamps))
		}
	}
	{
		td := &blockTestData{}
		td.GenerateTableLikeData(1000, 100)
		f(td)
	}
	{
		td := &blockTestData{}
		td.GenerateTableLikeDataWithDiffValueLen(1000, 100)
		f(td)
	}
	{
		td := &blockTestData{}
		td.GenerateRandomColCount(1000, 100)
		f(td)
	}
}
