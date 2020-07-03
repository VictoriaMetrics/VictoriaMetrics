package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestBlockStreamReaderSingleRow(t *testing.T) {
	rows := []rawRow{{
		Timestamp:     12334545,
		Value:         1.2345,
		PrecisionBits: defaultPrecisionBits,
	}}
	testBlocksStreamReader(t, rows, 1)
}

func TestBlockStreamReaderSingleBlockManyRows(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	for i := 0; i < maxRowsPerBlock; i++ {
		r.Value = rand.Float64()*1e9 - 5e8
		r.Timestamp = int64(i * 1e9)
		rows = append(rows, r)
	}
	testBlocksStreamReader(t, rows, 1)
}

func TestBlockStreamReaderSingleTSIDManyBlocks(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 1
	for i := 0; i < 5*maxRowsPerBlock; i++ {
		r.Value = rand.NormFloat64() * 1e4
		r.Timestamp = int64(rand.NormFloat64() * 1e9)
		rows = append(rows, r)
	}
	testBlocksStreamReader(t, rows, 5)
}

func TestBlockStreamReaderManyTSIDSingleRow(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	for i := 0; i < 1000; i++ {
		r.TSID.MetricID = uint64(i)
		r.Value = rand.Float64()*1e9 - 5e8
		r.Timestamp = int64(i * 1e9)
		rows = append(rows, r)
	}
	testBlocksStreamReader(t, rows, 1000)
}

func TestBlockStreamReaderManyTSIDManyRows(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	const blocks = 123
	for i := 0; i < 3210; i++ {
		r.TSID.MetricID = uint64((1e9 - i) % blocks)
		r.Value = rand.Float64()
		r.Timestamp = int64(rand.Float64() * 1e9)
		rows = append(rows, r)
	}
	testBlocksStreamReader(t, rows, blocks)
}

func TestBlockStreamReaderReadConcurrent(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	const blocks = 123
	for i := 0; i < 3210; i++ {
		r.TSID.MetricID = uint64((1e9 - i) % blocks)
		r.Value = rand.Float64()
		r.Timestamp = int64(rand.Float64() * 1e9)
		rows = append(rows, r)
	}
	var mp inmemoryPart
	mp.InitFromRows(rows)

	ch := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ch <- testBlockStreamReaderReadRows(&mp, rows)
		}()
	}
	for i := 0; i < 5; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second * 5):
			t.Fatalf("timeout")
		}
	}
}

func testBlockStreamReaderReadRows(mp *inmemoryPart, rows []rawRow) error {
	var bsr blockStreamReader
	bsr.InitFromInmemoryPart(mp)
	rowsCount := 0
	for bsr.NextBlock() {
		if err := bsr.Block.UnmarshalData(); err != nil {
			return fmt.Errorf("cannot unmarshal block data: %w", err)
		}
		for bsr.Block.nextRow() {
			rowsCount++
		}
	}
	if err := bsr.Error(); err != nil {
		return fmt.Errorf("unexpected error in bsr.NextBlock: %w", err)
	}
	if rowsCount != len(rows) {
		return fmt.Errorf("unexpected number of rows read; got %d; want %d", rowsCount, len(rows))
	}
	return nil
}

func testBlocksStreamReader(t *testing.T, rows []rawRow, expectedBlocksCount int) {
	t.Helper()

	bsr := newTestBlockStreamReader(t, rows)
	blocksCount := 0
	rowsCount := 0
	for bsr.NextBlock() {
		if err := bsr.Block.UnmarshalData(); err != nil {
			t.Fatalf("cannot unmarshal block data: %s", err)
		}
		for bsr.Block.nextRow() {
			rowsCount++
		}
		blocksCount++
	}
	if err := bsr.Error(); err != nil {
		t.Fatalf("unexpected error in bsr.NextBlock: %s", err)
	}
	if blocksCount != expectedBlocksCount {
		t.Fatalf("unexpected number of blocks read; got %d; want %d", blocksCount, expectedBlocksCount)
	}
	if rowsCount != len(rows) {
		t.Fatalf("unexpected number of rows read; got %d; want %d", rowsCount, len(rows))
	}
}

func newTestBlockStreamReader(t *testing.T, rows []rawRow) *blockStreamReader {
	var mp inmemoryPart
	mp.InitFromRows(rows)
	var bsr blockStreamReader
	bsr.InitFromInmemoryPart(&mp)
	return &bsr
}
