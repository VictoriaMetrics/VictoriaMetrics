package storage

import (
	"math/rand"
	"testing"
)

const defaultPrecisionBits = 4

func TestInmemoryPartInitFromRows(t *testing.T) {
	testInmemoryPartInitFromRows(t, []rawRow{
		{
			TSID: TSID{
				MetricID: 234,
			},
			Timestamp:     123,
			Value:         456.789,
			PrecisionBits: defaultPrecisionBits,
		},
	}, 1)

	var rows []rawRow
	var r rawRow

	// Test a single tsid.
	rows = rows[:0]
	initTestTSID(&r.TSID)
	r.PrecisionBits = defaultPrecisionBits
	for i := uint64(0); i < 1e4; i++ {
		r.Timestamp = int64(rand.NormFloat64() * 1e7)
		r.Value = rand.NormFloat64() * 100

		rows = append(rows, r)
	}
	testInmemoryPartInitFromRows(t, rows, 2)

	// Test distinct tsids.
	rows = rows[:0]
	for i := 0; i < 1e4; i++ {
		initTestTSID(&r.TSID)
		r.TSID.MetricID = uint64(i)
		r.Timestamp = int64(rand.NormFloat64() * 1e7)
		r.Value = rand.NormFloat64() * 100
		r.PrecisionBits = uint8(i%64) + 1

		rows = append(rows, r)
	}
	testInmemoryPartInitFromRows(t, rows, 1e4)
}

func testInmemoryPartInitFromRows(t *testing.T, rows []rawRow, blocksCount int) {
	t.Helper()

	minTimestamp := int64((1 << 63) - 1)
	maxTimestamp := int64(-1 << 63)
	for i := range rows {
		r := &rows[i]
		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}

	var mp inmemoryPart
	mp.InitFromRows(rows)

	if int(mp.ph.RowsCount) != len(rows) {
		t.Fatalf("unexpected rows count; got %d; expecting %d", mp.ph.RowsCount, len(rows))
	}
	if mp.ph.MinTimestamp != minTimestamp {
		t.Fatalf("unexpected minTimestamp; got %d; expecting %d", mp.ph.MinTimestamp, minTimestamp)
	}
	if mp.ph.MaxTimestamp != maxTimestamp {
		t.Fatalf("unexpected maxTimestamp; got %d; expecting %d", mp.ph.MaxTimestamp, maxTimestamp)
	}

	var bsr blockStreamReader
	bsr.InitFromInmemoryPart(&mp)

	rowsCount := 0
	blockNum := 0
	prevTSID := TSID{}
	for bsr.NextBlock() {
		bh := &bsr.Block.bh

		if bh.TSID.Less(&prevTSID) {
			t.Fatalf("TSID=%+v for the current block cannot be smaller than the TSID=%+v for the previous block", &bh.TSID, &prevTSID)
		}
		prevTSID = bh.TSID

		if bh.MinTimestamp < minTimestamp {
			t.Fatalf("unexpected MinTimestamp in the block %+v; got %d; cannot be smaller than %d", &bsr.Block, bh.MinTimestamp, minTimestamp)
		}
		if bh.MaxTimestamp > maxTimestamp {
			t.Fatalf("unexpected MaxTimestamp in the block %+v; got %d; cannot be higher than %d", &bsr.Block, bh.MaxTimestamp, maxTimestamp)
		}

		if err := bsr.Block.UnmarshalData(); err != nil {
			t.Fatalf("cannot unmarshal block #%d: %s", blockNum, err)
		}

		prevTimestamp := bh.MinTimestamp
		blockRowsCount := 0
		for bsr.Block.nextRow() {
			timestamp := bsr.Block.timestamps[bsr.Block.nextIdx-1]
			if timestamp < bh.MinTimestamp {
				t.Fatalf("unexpected Timestamp in the row; got %d; cannot be smaller than %d", timestamp, bh.MinTimestamp)
			}
			if timestamp > bsr.Block.bh.MaxTimestamp {
				t.Fatalf("unexpected Timestamp in the row; got %d; cannot be higher than %d", timestamp, bh.MaxTimestamp)
			}
			if timestamp < prevTimestamp {
				t.Fatalf("too small Timestamp in the row; got %d; cannot be smaller than the timestamp from the previous row: %d",
					timestamp, prevTimestamp)
			}
			prevTimestamp = timestamp
			blockRowsCount++
		}
		if blockRowsCount != int(bh.RowsCount) {
			t.Fatalf("unexpected number of rows in the block %v; got %d; want %d", &bsr.Block, blockRowsCount, bh.RowsCount)
		}

		rowsCount += blockRowsCount
		blockNum++
	}
	if err := bsr.Error(); err != nil {
		t.Fatalf("unexpected error after reading %d blocks from block stream: %s", blockNum, err)
	}
	if blockNum != blocksCount {
		t.Fatalf("unexpected number of blocks read; got %d; want %d", blockNum, blocksCount)
	}
	if rowsCount != len(rows) {
		t.Fatalf("unexpected number of rows; got %d; want %d", rowsCount, len(rows))
	}
}
