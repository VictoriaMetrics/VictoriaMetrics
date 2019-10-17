package storage

import (
	"math/rand"
	"testing"
)

func TestMergeBlockStreamsOneStreamOneRow(t *testing.T) {
	rows := []rawRow{
		{
			Timestamp:     82394327423432,
			Value:         123.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr := newTestBlockStreamReader(t, rows)
	bsrs := []*blockStreamReader{bsr}
	testMergeBlockStreams(t, bsrs, 1, 1, rows[0].Timestamp, rows[0].Timestamp)
}

func TestMergeBlockStreamsOneStreamOneBlockManyRows(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 4
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)
	for i := 0; i < maxRowsPerBlock; i++ {
		r.Timestamp = int64(rand.Intn(1e9))
		r.Value = rand.NormFloat64() * 2332
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr := newTestBlockStreamReader(t, rows)
	bsrs := []*blockStreamReader{bsr}
	testMergeBlockStreams(t, bsrs, 1, maxRowsPerBlock, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsOneStreamManyBlocksOneRow(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 4
	const blocksCount = 1234
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)
	for i := 0; i < blocksCount; i++ {
		initTestTSID(&r.TSID)
		r.TSID.MetricID = uint64(i * 123)
		r.Timestamp = int64(rand.Intn(1e9))
		r.Value = rand.NormFloat64() * 2332
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr := newTestBlockStreamReader(t, rows)
	bsrs := []*blockStreamReader{bsr}
	testMergeBlockStreams(t, bsrs, blocksCount, blocksCount, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsOneStreamManyBlocksManyRows(t *testing.T) {
	var rows []rawRow
	var r rawRow
	initTestTSID(&r.TSID)
	r.PrecisionBits = 4
	const blocksCount = 1234
	const rowsCount = 4938
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)
	for i := 0; i < rowsCount; i++ {
		r.TSID.MetricID = uint64(i % blocksCount)
		r.Timestamp = int64(rand.Intn(1e9))
		r.Value = rand.NormFloat64() * 2332
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr := newTestBlockStreamReader(t, rows)
	bsrs := []*blockStreamReader{bsr}
	testMergeBlockStreams(t, bsrs, blocksCount, rowsCount, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsTwoStreamsOneBlockTwoRows(t *testing.T) {
	// Identical rows
	rows := []rawRow{
		{
			Timestamp:     182394327423432,
			Value:         3123.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr1 := newTestBlockStreamReader(t, rows)
	bsr2 := newTestBlockStreamReader(t, rows)
	bsrs := []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, 1, 2, rows[0].Timestamp, rows[0].Timestamp)

	// Distinct rows for the same TSID.
	minTimestamp := int64(12332443)
	maxTimestamp := int64(23849834543)
	rows = []rawRow{
		{
			Timestamp:     maxTimestamp,
			Value:         3123.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr1 = newTestBlockStreamReader(t, rows)
	rows = []rawRow{
		{
			Timestamp:     minTimestamp,
			Value:         23.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr2 = newTestBlockStreamReader(t, rows)
	bsrs = []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, 1, 2, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsTwoStreamsTwoBlocksOneRow(t *testing.T) {
	minTimestamp := int64(4389345)
	maxTimestamp := int64(8394584354)

	rows := []rawRow{
		{
			TSID: TSID{
				MetricID: 8454,
			},
			Timestamp:     minTimestamp,
			Value:         33.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr1 := newTestBlockStreamReader(t, rows)

	rows = []rawRow{
		{
			TSID: TSID{
				MetricID: 4454,
			},
			Timestamp:     maxTimestamp,
			Value:         323.42389,
			PrecisionBits: defaultPrecisionBits,
		},
	}
	bsr2 := newTestBlockStreamReader(t, rows)

	bsrs := []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, 2, 2, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsTwoStreamsManyBlocksManyRows(t *testing.T) {
	const blocksCount = 1234
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)

	var rows []rawRow
	var r rawRow
	initTestTSID(&r.TSID)
	r.PrecisionBits = 2
	const rowsCount1 = 4938
	for i := 0; i < rowsCount1; i++ {
		r.TSID.MetricID = uint64(i % blocksCount)
		r.Timestamp = int64(rand.Intn(1e9))
		r.Value = rand.NormFloat64() * 2332
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr1 := newTestBlockStreamReader(t, rows)

	rows = rows[:0]
	const rowsCount2 = 3281
	for i := 0; i < rowsCount2; i++ {
		r.TSID.MetricID = uint64((i + 17) % blocksCount)
		r.Timestamp = int64(rand.Intn(1e9))
		r.Value = rand.NormFloat64() * 2332
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr2 := newTestBlockStreamReader(t, rows)

	bsrs := []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, blocksCount, rowsCount1+rowsCount2, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsTwoStreamsBigOverlappingBlocks(t *testing.T) {
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)

	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 5
	const rowsCount1 = maxRowsPerBlock + 234
	for i := 0; i < rowsCount1; i++ {
		r.Timestamp = int64(i * 2894)
		r.Value = float64(int(rand.NormFloat64() * 1e2))
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr1 := newTestBlockStreamReader(t, rows)

	rows = rows[:0]
	const rowsCount2 = maxRowsPerBlock + 2344
	for i := 0; i < rowsCount2; i++ {
		r.Timestamp = int64(i * 2494)
		r.Value = float64(int(rand.NormFloat64() * 1e2))
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr2 := newTestBlockStreamReader(t, rows)

	bsrs := []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, 3, rowsCount1+rowsCount2, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsTwoStreamsBigSequentialBlocks(t *testing.T) {
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)

	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 5
	const rowsCount1 = maxRowsPerBlock + 234
	for i := 0; i < rowsCount1; i++ {
		r.Timestamp = int64(i * 2894)
		r.Value = float64(int(rand.NormFloat64() * 1e2))
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	maxTimestampB1 := rows[len(rows)-1].Timestamp
	bsr1 := newTestBlockStreamReader(t, rows)

	rows = rows[:0]
	const rowsCount2 = maxRowsPerBlock - 233
	for i := 0; i < rowsCount2; i++ {
		r.Timestamp = maxTimestampB1 + int64(i*2494)
		r.Value = float64(int(rand.NormFloat64() * 1e2))
		rows = append(rows, r)

		if r.Timestamp < minTimestamp {
			minTimestamp = r.Timestamp
		}
		if r.Timestamp > maxTimestamp {
			maxTimestamp = r.Timestamp
		}
	}
	bsr2 := newTestBlockStreamReader(t, rows)

	bsrs := []*blockStreamReader{bsr1, bsr2}
	testMergeBlockStreams(t, bsrs, 3, rowsCount1+rowsCount2, minTimestamp, maxTimestamp)
}

func TestMergeBlockStreamsManyStreamsManyBlocksManyRows(t *testing.T) {
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)

	var r rawRow
	initTestTSID(&r.TSID)
	r.PrecisionBits = defaultPrecisionBits

	rowsCount := 0
	const blocksCount = 113
	var bsrs []*blockStreamReader
	for i := 0; i < 20; i++ {
		rowsPerStream := rand.Intn(500)
		var rows []rawRow
		for j := 0; j < rowsPerStream; j++ {
			r.TSID.MetricID = uint64(j % blocksCount)
			r.Timestamp = int64(rand.Intn(1e9))
			r.Value = rand.NormFloat64()
			rows = append(rows, r)

			if r.Timestamp < minTimestamp {
				minTimestamp = r.Timestamp
			}
			if r.Timestamp > maxTimestamp {
				maxTimestamp = r.Timestamp
			}
		}
		bsr := newTestBlockStreamReader(t, rows)
		bsrs = append(bsrs, bsr)
		rowsCount += rowsPerStream
	}
	testMergeBlockStreams(t, bsrs, blocksCount, rowsCount, minTimestamp, maxTimestamp)
}

func TestMergeForciblyStop(t *testing.T) {
	minTimestamp := int64(1<<63 - 1)
	maxTimestamp := int64(-1 << 63)

	var r rawRow
	initTestTSID(&r.TSID)
	r.PrecisionBits = defaultPrecisionBits

	const blocksCount = 113
	var bsrs []*blockStreamReader
	for i := 0; i < 20; i++ {
		rowsPerStream := rand.Intn(1000)
		var rows []rawRow
		for j := 0; j < rowsPerStream; j++ {
			r.TSID.MetricID = uint64(j % blocksCount)
			r.Timestamp = int64(rand.Intn(1e9))
			r.Value = rand.NormFloat64()
			rows = append(rows, r)

			if r.Timestamp < minTimestamp {
				minTimestamp = r.Timestamp
			}
			if r.Timestamp > maxTimestamp {
				maxTimestamp = r.Timestamp
			}
		}
		bsr := newTestBlockStreamReader(t, rows)
		bsrs = append(bsrs, bsr)
	}

	var mp inmemoryPart
	var bsw blockStreamWriter
	bsw.InitFromInmemoryPart(&mp)
	ch := make(chan struct{})
	var rowsMerged, rowsDeleted uint64
	close(ch)
	if err := mergeBlockStreams(&mp.ph, &bsw, bsrs, ch, &rowsMerged, nil, &rowsDeleted); err != errForciblyStopped {
		t.Fatalf("unexpected error in mergeBlockStreams: got %v; want %v", err, errForciblyStopped)
	}
	if rowsMerged != 0 {
		t.Fatalf("unexpected rowsMerged; got %d; want %d", rowsMerged, 0)
	}
	if rowsDeleted != 0 {
		t.Fatalf("unexpected rowsDeleted; got %d; want %d", rowsDeleted, 0)
	}
}

func testMergeBlockStreams(t *testing.T, bsrs []*blockStreamReader, expectedBlocksCount, expectedRowsCount int, expectedMinTimestamp, expectedMaxTimestamp int64) {
	t.Helper()

	var mp inmemoryPart

	var bsw blockStreamWriter
	bsw.InitFromInmemoryPart(&mp)

	var rowsMerged, rowsDeleted uint64
	if err := mergeBlockStreams(&mp.ph, &bsw, bsrs, nil, &rowsMerged, nil, &rowsDeleted); err != nil {
		t.Fatalf("unexpected error in mergeBlockStreams: %s", err)
	}

	// Verify written data.
	if mp.ph.RowsCount != uint64(expectedRowsCount) {
		t.Fatalf("unexpected rows count in partHeader; got %d; want %d", mp.ph.RowsCount, expectedRowsCount)
	}
	if rowsMerged != mp.ph.RowsCount {
		t.Fatalf("unexpected rowsMerged; got %d; want %d", rowsMerged, mp.ph.RowsCount)
	}
	if rowsDeleted != 0 {
		t.Fatalf("unexpected rowsDeleted; got %d; want %d", rowsDeleted, 0)
	}
	if mp.ph.MinTimestamp != expectedMinTimestamp {
		t.Fatalf("unexpected MinTimestamp in partHeader; got %d; want %d", mp.ph.MinTimestamp, expectedMinTimestamp)
	}
	if mp.ph.MaxTimestamp != expectedMaxTimestamp {
		t.Fatalf("unexpected MaxTimestamp in partHeader; got %d; want %d", mp.ph.MaxTimestamp, expectedMaxTimestamp)
	}

	var bsr1 blockStreamReader
	bsr1.InitFromInmemoryPart(&mp)
	blocksCount := 0
	rowsCount := 0
	var prevTSID TSID
	for bsr1.NextBlock() {
		if bsr1.Block.bh.TSID.Less(&prevTSID) {
			t.Fatalf("the next block cannot have higher TSID than the previous block; got\n%+v vs\n%+v", &bsr1.Block.bh.TSID, &prevTSID)
		}
		prevTSID = bsr1.Block.bh.TSID

		expectedRowsPerBlock := int(bsr1.Block.bh.RowsCount)
		if expectedRowsPerBlock == 0 {
			t.Fatalf("got zero rows in a block")
		}
		if bsr1.Block.bh.MinTimestamp < expectedMinTimestamp {
			t.Fatalf("too small MinTimestamp in the blockHeader; got %d; cannot be smaller than %d", bsr1.Block.bh.MinTimestamp, expectedMinTimestamp)
		}
		if bsr1.Block.bh.MaxTimestamp > expectedMaxTimestamp {
			t.Fatalf("too big MaxTimestamp in the blockHeader; got %d; cannot be bigger than %d", bsr1.Block.bh.MaxTimestamp, expectedMaxTimestamp)
		}

		if err := bsr1.Block.UnmarshalData(); err != nil {
			t.Fatalf("cannot unmarshal block from merged stream: %s", err)
		}

		prevTimestamp := bsr1.Block.bh.MinTimestamp
		blockMaxTimestamp := bsr1.Block.bh.MaxTimestamp
		rowsPerBlock := 0
		for bsr1.Block.nextRow() {
			rowsPerBlock++
			timestamp := bsr1.Block.timestamps[bsr1.Block.nextIdx-1]
			if timestamp < prevTimestamp {
				t.Fatalf("the next timestamp cannot be smaller than the previous timestamp; got %d vs %d", timestamp, prevTimestamp)
			}
			prevTimestamp = timestamp
		}
		if prevTimestamp > blockMaxTimestamp {
			t.Fatalf("the last timestamp cannot be bigger than the MaxTimestamp in the blockHeader; got %d vs %d", prevTimestamp, blockMaxTimestamp)
		}
		if rowsPerBlock != expectedRowsPerBlock {
			t.Fatalf("unexpected rows read in the block; got %d; want %d", rowsPerBlock, expectedRowsPerBlock)
		}
		rowsCount += rowsPerBlock
		blocksCount++
	}
	if err := bsr1.Error(); err != nil {
		t.Fatalf("unexpected error when reading merged stream: %s", err)
	}
	if blocksCount != expectedBlocksCount {
		t.Fatalf("unexpected blocks read from merged stream; got %d; want %d", blocksCount, expectedBlocksCount)
	}
	if rowsCount != expectedRowsCount {
		t.Fatalf("unexpected rows read from merged stream; got %d; want %d", rowsCount, expectedRowsCount)
	}
}
