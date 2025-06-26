package logstorage

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestInmemoryPartMustInitFromRows(t *testing.T) {
	f := func(lrOrig *LogRows, blocksCountExpected int, compressionRateExpected float64) {
		t.Helper()

		uncompressedSizeBytesExpected := uncompressedRowsSizeBytes(lrOrig.rows)
		rowsCountExpected := len(lrOrig.timestamps)
		minTimestampExpected := int64(math.MaxInt64)
		maxTimestampExpected := int64(math.MinInt64)

		for _, timestamp := range lrOrig.timestamps {
			if timestamp < minTimestampExpected {
				minTimestampExpected = timestamp
			}
			if timestamp > maxTimestampExpected {
				maxTimestampExpected = timestamp
			}
		}

		var lrExpected logRows
		lrExpected.mustAddRows(lrOrig)

		var lr logRows
		lr.mustAddRows(lrOrig)

		// Create inmemory part from lr
		mp := getInmemoryPart()
		mp.mustInitFromRows(&lr)

		// Check mp.ph
		ph := &mp.ph
		checkCompressionRate(t, ph, compressionRateExpected)
		if ph.UncompressedSizeBytes != uncompressedSizeBytesExpected {
			t.Fatalf("unexpected UncompressedSizeBytes in partHeader; got %d; want %d", ph.UncompressedSizeBytes, uncompressedSizeBytesExpected)
		}
		if ph.RowsCount != uint64(rowsCountExpected) {
			t.Fatalf("unexpected rowsCount in partHeader; got %d; want %d", ph.RowsCount, rowsCountExpected)
		}
		if ph.BlocksCount != uint64(blocksCountExpected) {
			t.Fatalf("unexpected blocksCount in partHeader; got %d; want %d", ph.BlocksCount, blocksCountExpected)
		}
		if ph.RowsCount > 0 {
			if ph.MinTimestamp != minTimestampExpected {
				t.Fatalf("unexpected minTimestamp in partHeader; got %d; want %d", ph.MinTimestamp, minTimestampExpected)
			}
			if ph.MaxTimestamp != maxTimestampExpected {
				t.Fatalf("unexpected maxTimestamp in partHeader; got %d; want %d", ph.MaxTimestamp, maxTimestampExpected)
			}
		}

		// Read log entries from mp to lrResult
		sbu := getStringsBlockUnmarshaler()
		defer putStringsBlockUnmarshaler(sbu)
		vd := getValuesDecoder()
		defer putValuesDecoder(vd)
		lrResult := mp.readLogRows(sbu, vd)
		putInmemoryPart(mp)

		// compare lrExpected to lrResult
		if err := checkEqualRows(lrResult, &lrExpected); err != nil {
			t.Fatalf("unequal log entries: %s", err)
		}
	}

	f(GetLogRows(nil, nil, nil, nil, ""), 0, 0)

	// Check how inmemoryPart works with a single stream
	f(newTestLogRows(1, 1, 0), 1, 1.5)
	f(newTestLogRows(1, 2, 0), 1, 1.7)
	f(newTestLogRows(1, 10, 0), 1, 4.6)
	f(newTestLogRows(1, 1000, 0), 1, 17.1)
	f(newTestLogRows(1, 20000, 0), 6, 16.8)

	// Check how inmemoryPart works with multiple streams
	f(newTestLogRows(2, 1, 0), 2, 1.8)
	f(newTestLogRows(10, 1, 0), 10, 2.1)
	f(newTestLogRows(100, 1, 0), 100, 2.3)
	f(newTestLogRows(10, 5, 0), 10, 3.6)
	f(newTestLogRows(10, 1000, 0), 10, 17.1)
	f(newTestLogRows(100, 100, 0), 100, 13)
}

func TestInmemoryPartMustInitFromRows_Overflow(t *testing.T) {
	f := func(lrOrig *LogRows, blocksCountExpected int, compressionRateExpected float64) {
		t.Helper()

		var lr logRows
		lr.mustAddRows(lrOrig)

		// Create inmemory part from lr
		mp := getInmemoryPart()
		mp.mustInitFromRows(&lr)

		// Check mp.ph
		ph := &mp.ph
		checkCompressionRate(t, ph, compressionRateExpected)
		if ph.BlocksCount != uint64(blocksCountExpected) {
			t.Fatalf("unexpected blocksCount in partHeader; got %d; want %d", ph.BlocksCount, blocksCountExpected)
		}
	}

	// check block overflow with unique tag rows
	f(newTestLogRowsUniqTags(5, 21, 100), 5, 0.6)
	f(newTestLogRowsUniqTags(5, 10, 100), 5, 0.7)
	f(newTestLogRowsUniqTags(1, 2001, 1), 1, 2.0)
	f(newTestLogRowsUniqTags(15, 20, 250), 15, 0.8)
}

func checkCompressionRate(t *testing.T, ph *partHeader, compressionRateExpected float64) {
	t.Helper()
	compressionRate := float64(ph.UncompressedSizeBytes) / float64(ph.CompressedSizeBytes)
	if math.Abs(compressionRate-compressionRateExpected) > math.Abs(compressionRate+compressionRateExpected)*0.05 {
		t.Fatalf("unexpected compression rate; got %.1f; want %.1f", compressionRate, compressionRateExpected)
	}
}

func TestInmemoryPartInitFromBlockStreamReaders(t *testing.T) {
	f := func(lrs []*LogRows, blocksCountExpected int, compressionRateExpected float64) {
		t.Helper()

		uncompressedSizeBytesExpected := uint64(0)
		rowsCountExpected := 0
		minTimestampExpected := int64(math.MaxInt64)
		maxTimestampExpected := int64(math.MinInt64)

		// make a copy of lrs in order to compare the results after merge.
		var lrExpected logRows
		for _, lr := range lrs {
			uncompressedSizeBytesExpected += uncompressedRowsSizeBytes(lr.rows)
			rowsCountExpected += len(lr.timestamps)
			for j, timestamp := range lr.timestamps {
				if timestamp < minTimestampExpected {
					minTimestampExpected = timestamp
				}
				if timestamp > maxTimestampExpected {
					maxTimestampExpected = timestamp
				}
				lrExpected.mustAddRow(lr.streamIDs[j], timestamp, lr.rows[j])
			}
		}

		// Initialize readers from lrs
		var mpsSrc []*inmemoryPart
		var bsrs []*blockStreamReader
		for _, lrOrig := range lrs {
			var lr logRows
			lr.mustAddRows(lrOrig)

			mp := getInmemoryPart()
			mp.mustInitFromRows(&lr)
			mpsSrc = append(mpsSrc, mp)

			bsr := getBlockStreamReader()
			bsr.MustInitFromInmemoryPart(mp)
			bsrs = append(bsrs, bsr)
		}
		defer func() {
			for _, bsr := range bsrs {
				putBlockStreamReader(bsr)
			}
			for _, mp := range mpsSrc {
				putInmemoryPart(mp)
			}
		}()

		// Merge data from bsrs into mpDst
		mpDst := getInmemoryPart()
		bsw := getBlockStreamWriter()
		bsw.MustInitForInmemoryPart(mpDst)
		mustMergeBlockStreams(&mpDst.ph, bsw, bsrs, nil)
		putBlockStreamWriter(bsw)

		// Check mpDst.ph stats
		ph := &mpDst.ph
		checkCompressionRate(t, ph, compressionRateExpected)
		if ph.UncompressedSizeBytes != uncompressedSizeBytesExpected {
			t.Fatalf("unexpected UncompressedSizeBytes in partHeader; got %d; want %d", ph.UncompressedSizeBytes, uncompressedSizeBytesExpected)
		}
		if ph.RowsCount != uint64(rowsCountExpected) {
			t.Fatalf("unexpected number of entries in partHeader; got %d; want %d", ph.RowsCount, rowsCountExpected)
		}
		if ph.BlocksCount != uint64(blocksCountExpected) {
			t.Fatalf("unexpected blocksCount in partHeader; got %d; want %d", ph.BlocksCount, blocksCountExpected)
		}
		if ph.RowsCount > 0 {
			if ph.MinTimestamp != minTimestampExpected {
				t.Fatalf("unexpected minTimestamp in partHeader; got %d; want %d", ph.MinTimestamp, minTimestampExpected)
			}
			if ph.MaxTimestamp != maxTimestampExpected {
				t.Fatalf("unexpected maxTimestamp in partHeader; got %d; want %d", ph.MaxTimestamp, maxTimestampExpected)
			}
		}

		// Read log entries from mpDst to lrResult
		sbu := getStringsBlockUnmarshaler()
		defer putStringsBlockUnmarshaler(sbu)
		vd := getValuesDecoder()
		defer putValuesDecoder(vd)
		lrResult := mpDst.readLogRows(sbu, vd)
		putInmemoryPart(mpDst)

		// compare lrExpected to lrResult
		if err := checkEqualRows(lrResult, &lrExpected); err != nil {
			t.Fatalf("unequal log entries: %s", err)
		}
	}

	// Check empty readers
	f(nil, 0, 0)
	f([]*LogRows{GetLogRows(nil, nil, nil, nil, "")}, 0, 0)
	f([]*LogRows{GetLogRows(nil, nil, nil, nil, ""), GetLogRows(nil, nil, nil, nil, "")}, 0, 0)

	// Check merge with a single reader
	f([]*LogRows{newTestLogRows(1, 1, 0)}, 1, 1.5)
	f([]*LogRows{newTestLogRows(1, 10, 0)}, 1, 4.6)
	f([]*LogRows{newTestLogRows(1, 100, 0)}, 1, 13.0)
	f([]*LogRows{newTestLogRows(1, 1000, 0)}, 1, 17.1)
	f([]*LogRows{newTestLogRows(1, 10000, 0)}, 3, 17.2)
	f([]*LogRows{newTestLogRows(10, 1, 0)}, 10, 2.1)
	f([]*LogRows{newTestLogRows(100, 1, 0)}, 100, 2.3)
	f([]*LogRows{newTestLogRows(1000, 1, 0)}, 1000, 2.4)
	f([]*LogRows{newTestLogRows(10, 10, 0)}, 10, 5.5)
	f([]*LogRows{newTestLogRows(10, 100, 0)}, 10, 13)

	//Check merge with multiple readers
	f([]*LogRows{
		newTestLogRows(1, 1, 0),
		newTestLogRows(1, 1, 1),
	}, 2, 1.7)
	f([]*LogRows{
		newTestLogRows(2, 2, 0),
		newTestLogRows(2, 2, 0),
	}, 2, 4.2)
	f([]*LogRows{
		newTestLogRows(1, 20, 0),
		newTestLogRows(1, 10, 1),
		newTestLogRows(1, 5, 2),
	}, 3, 5.5)
	f([]*LogRows{
		newTestLogRows(10, 20, 0),
		newTestLogRows(20, 10, 1),
		newTestLogRows(30, 5, 2),
	}, 60, 5.2)
	f([]*LogRows{
		newTestLogRows(10, 20, 0),
		newTestLogRows(20, 10, 1),
		newTestLogRows(30, 5, 2),
		newTestLogRows(20, 7, 3),
		newTestLogRows(10, 9, 4),
	}, 90, 5.0)
}

func newTestLogRows(streams, rowsPerStream int, seed int64) *LogRows {
	longConstValue := "some-value " + string(make([]byte, maxConstColumnValueSize))
	streamTags := []string{
		"some-stream-tag",
	}
	lr := GetLogRows(streamTags, nil, nil, nil, "")
	rng := rand.New(rand.NewSource(seed))
	var fields []Field
	for i := 0; i < streams; i++ {
		tenantID := TenantID{
			AccountID: rng.Uint32(),
			ProjectID: rng.Uint32(),
		}
		for j := 0; j < rowsPerStream; j++ {
			// Add stream tags
			fields = append(fields[:0], Field{
				Name:  "some-stream-tag",
				Value: fmt.Sprintf("some-stream-value-%d", i),
			})
			// Add the remaining tags
			for k := 0; k < 5; k++ {
				if rng.Float64() < 0.5 {
					fields = append(fields, Field{
						Name:  fmt.Sprintf("field_%d", k),
						Value: fmt.Sprintf("value_%d_%d_%d", i, j, k),
					})
				}
			}
			// add a message field
			fields = append(fields, Field{
				Name:  "",
				Value: fmt.Sprintf("some row number %d at stream %d", j, i),
			})
			// add a field with constant value
			fields = append(fields, Field{
				Name:  "job",
				Value: "foobar",
			})
			// add a field with const value with the length exceeding maxConstColumnValueSize
			fields = append(fields, Field{
				Name:  "long-const",
				Value: longConstValue,
			})
			// add a field with uint value
			fields = append(fields, Field{
				Name:  "response_size_bytes",
				Value: fmt.Sprintf("%d", rng.Intn(1234)),
			})
			// shuffle fields in order to check de-shuffling algorithm
			rng.Shuffle(len(fields), func(i, j int) {
				fields[i], fields[j] = fields[j], fields[i]
			})
			timestamp := rng.Int63()
			lr.MustAdd(tenantID, timestamp, fields, nil)
		}
	}
	return lr
}

func checkEqualRows(lrResult, lrOrig *logRows) error {
	if len(lrResult.timestamps) != len(lrOrig.timestamps) {
		return fmt.Errorf("unexpected length LogRows; got %d; want %d", len(lrResult.timestamps), len(lrOrig.timestamps))
	}

	sort.Sort(lrResult)
	sort.Sort(lrOrig)

	for i := range lrOrig.timestamps {
		if !lrOrig.streamIDs[i].equal(&lrResult.streamIDs[i]) {
			return fmt.Errorf("unexpected streamID for log entry %d\ngot\n%s\nwant\n%s", i, &lrResult.streamIDs[i], &lrOrig.streamIDs[i])
		}
		if lrOrig.timestamps[i] != lrResult.timestamps[i] {
			return fmt.Errorf("unexpected timestamp for log entry %d\ngot\n%d\nwant\n%d", i, lrResult.timestamps[i], lrOrig.timestamps[i])
		}
		fieldsOrig := lrOrig.rows[i]
		fieldsResult := lrResult.rows[i]
		if len(fieldsOrig) != len(fieldsResult) {
			return fmt.Errorf("unexpected number of fields at log entry %d\ngot\n%s\nwant\n%s", i, fieldsResult, fieldsOrig)
		}
		sortFieldsByName(fieldsOrig)
		sortFieldsByName(fieldsResult)
		if !reflect.DeepEqual(fieldsOrig, fieldsResult) {
			return fmt.Errorf("unexpected fields for log entry %d\ngot\n%s\nwant\n%s", i, fieldsResult, fieldsOrig)
		}
	}
	return nil
}

// readLogRows reads log entries from mp.
//
// This function is for testing and debugging purposes only.
func (mp *inmemoryPart) readLogRows(sbu *stringsBlockUnmarshaler, vd *valuesDecoder) *logRows {
	var lr logRows

	bsr := getBlockStreamReader()
	defer putBlockStreamReader(bsr)
	bsr.MustInitFromInmemoryPart(mp)
	var tmp rows
	for bsr.NextBlock() {
		bd := &bsr.blockData
		streamID := bd.streamID
		if err := bd.unmarshalRows(&tmp, sbu, vd); err != nil {
			logger.Panicf("BUG: cannot unmarshal log entries from inmemoryPart: %s", err)
		}
		for i, timestamp := range tmp.timestamps {
			lr.mustAddRow(streamID, timestamp, tmp.rows[i])
			lr.streamIDs[len(lr.streamIDs)-1] = streamID
		}
		tmp.reset()
	}
	return &lr
}

func newTestLogRowsUniqTags(streams, rowsPerStream, uniqFieldsPerRow int) *LogRows {
	streamTags := []string{
		"some-stream-tag",
	}
	lr := GetLogRows(streamTags, nil, nil, nil, "")
	var fields []Field
	for i := 0; i < streams; i++ {
		tenantID := TenantID{
			AccountID: 0,
			ProjectID: 0,
		}
		for j := 0; j < rowsPerStream; j++ {
			// Add stream tags
			fields = append(fields[:0], Field{
				Name:  "some-stream-tag",
				Value: fmt.Sprintf("some-stream-value-%d", i),
			})
			// Add the remaining unique tags
			for k := 0; k < uniqFieldsPerRow; k++ {
				fields = append(fields, Field{
					Name:  fmt.Sprintf("field_%d_%d_%d", i, j, k),
					Value: fmt.Sprintf("value_%d_%d_%d", i, j, k),
				})
			}
			lr.MustAdd(tenantID, time.Now().UnixMilli(), fields, nil)
		}
	}
	return lr
}
