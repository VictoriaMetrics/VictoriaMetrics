package logstorage

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestInmemoryPartMustInitFromRows(t *testing.T) {
	f := func(lr *LogRows, blocksCountExpected int, compressionRateExpected float64) {
		t.Helper()

		uncompressedSizeBytesExpected := uncompressedRowsSizeBytes(lr.rows)
		rowsCountExpected := len(lr.timestamps)
		minTimestampExpected := int64(math.MaxInt64)
		maxTimestampExpected := int64(math.MinInt64)

		// make a copy of lr - it is used for comapring the results later,
		// since lr may be modified by inmemoryPart.mustInitFromRows()
		lrOrig := GetLogRows(nil, nil)
		for i, timestamp := range lr.timestamps {
			if timestamp < minTimestampExpected {
				minTimestampExpected = timestamp
			}
			if timestamp > maxTimestampExpected {
				maxTimestampExpected = timestamp
			}
			lrOrig.mustAddInternal(lr.streamIDs[i], timestamp, lr.rows[i], lr.streamTagsCanonicals[i])
		}

		// Create inmemory part from lr
		mp := getInmemoryPart()
		mp.mustInitFromRows(lr)

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

		// Read log entries from mp to rrsResult
		sbu := getStringsBlockUnmarshaler()
		defer putStringsBlockUnmarshaler(sbu)
		vd := getValuesDecoder()
		defer putValuesDecoder(vd)
		lrResult := mp.readLogRows(sbu, vd)
		putInmemoryPart(mp)

		// compare lrOrig to lrResult
		if err := checkEqualRows(lrResult, lrOrig); err != nil {
			t.Fatalf("unequal log entries: %s", err)
		}
	}

	f(GetLogRows(nil, nil), 0, 0)

	// Check how inmemoryPart works with a single stream
	f(newTestLogRows(1, 1, 0), 1, 0.8)
	f(newTestLogRows(1, 2, 0), 1, 0.9)
	f(newTestLogRows(1, 10, 0), 1, 2.0)
	f(newTestLogRows(1, 1000, 0), 1, 7.1)
	f(newTestLogRows(1, 20000, 0), 2, 7.2)

	// Check how inmemoryPart works with multiple streams
	f(newTestLogRows(2, 1, 0), 2, 0.8)
	f(newTestLogRows(10, 1, 0), 10, 0.9)
	f(newTestLogRows(100, 1, 0), 100, 1.0)
	f(newTestLogRows(10, 5, 0), 10, 1.4)
	f(newTestLogRows(10, 1000, 0), 10, 7.2)
	f(newTestLogRows(100, 100, 0), 100, 5.0)
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

		// make a copy of rrss in order to compare the results after merge.
		lrOrig := GetLogRows(nil, nil)
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
				lrOrig.mustAddInternal(lr.streamIDs[j], timestamp, lr.rows[j], lr.streamTagsCanonicals[j])
			}
		}

		// Initialize readers from lrs
		var mpsSrc []*inmemoryPart
		var bsrs []*blockStreamReader
		for _, lr := range lrs {
			mp := getInmemoryPart()
			mp.mustInitFromRows(lr)
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
			t.Fatalf("unexpected uncompressedSizeBytes in partHeader; got %d; want %d", ph.UncompressedSizeBytes, uncompressedSizeBytesExpected)
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

		// Read log entries from mpDst to rrsResult
		sbu := getStringsBlockUnmarshaler()
		defer putStringsBlockUnmarshaler(sbu)
		vd := getValuesDecoder()
		defer putValuesDecoder(vd)
		lrResult := mpDst.readLogRows(sbu, vd)
		putInmemoryPart(mpDst)

		// compare rrsOrig to rrsResult
		if err := checkEqualRows(lrResult, lrOrig); err != nil {
			t.Fatalf("unequal log entries: %s", err)
		}
	}

	// Check empty readers
	f(nil, 0, 0)
	f([]*LogRows{GetLogRows(nil, nil)}, 0, 0)
	f([]*LogRows{GetLogRows(nil, nil), GetLogRows(nil, nil)}, 0, 0)

	// Check merge with a single reader
	f([]*LogRows{newTestLogRows(1, 1, 0)}, 1, 0.8)
	f([]*LogRows{newTestLogRows(1, 10, 0)}, 1, 2.0)
	f([]*LogRows{newTestLogRows(1, 100, 0)}, 1, 4.9)
	f([]*LogRows{newTestLogRows(1, 1000, 0)}, 1, 7.1)
	f([]*LogRows{newTestLogRows(1, 10000, 0)}, 1, 7.4)
	f([]*LogRows{newTestLogRows(10, 1, 0)}, 10, 0.9)
	f([]*LogRows{newTestLogRows(100, 1, 0)}, 100, 1.0)
	f([]*LogRows{newTestLogRows(1000, 1, 0)}, 1000, 1.0)
	f([]*LogRows{newTestLogRows(10, 10, 0)}, 10, 2.1)
	f([]*LogRows{newTestLogRows(10, 100, 0)}, 10, 4.9)

	//Check merge with multiple readers
	f([]*LogRows{
		newTestLogRows(1, 1, 0),
		newTestLogRows(1, 1, 1),
	}, 2, 0.9)
	f([]*LogRows{
		newTestLogRows(2, 2, 0),
		newTestLogRows(2, 2, 0),
	}, 2, 1.8)
	f([]*LogRows{
		newTestLogRows(1, 20, 0),
		newTestLogRows(1, 10, 1),
		newTestLogRows(1, 5, 2),
	}, 3, 2.2)
	f([]*LogRows{
		newTestLogRows(10, 20, 0),
		newTestLogRows(20, 10, 1),
		newTestLogRows(30, 5, 2),
	}, 60, 2.0)
	f([]*LogRows{
		newTestLogRows(10, 20, 0),
		newTestLogRows(20, 10, 1),
		newTestLogRows(30, 5, 2),
		newTestLogRows(20, 7, 3),
		newTestLogRows(10, 9, 4),
	}, 90, 1.9)
}

func newTestLogRows(streams, rowsPerStream int, seed int64) *LogRows {
	streamTags := []string{
		"some-stream-tag",
	}
	lr := GetLogRows(streamTags, nil)
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
			lr.MustAdd(tenantID, timestamp, fields)
		}
	}
	return lr
}

func checkEqualRows(lrResult, lrOrig *LogRows) error {
	if len(lrResult.timestamps) != len(lrOrig.timestamps) {
		return fmt.Errorf("unexpected length LogRows; got %d; want %d", len(lrResult.timestamps), len(lrOrig.timestamps))
	}

	sort.Sort(lrResult)
	sort.Sort(lrOrig)

	sortFieldNames := func(fields []Field) {
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].Name < fields[j].Name
		})
	}
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
		sortFieldNames(fieldsOrig)
		sortFieldNames(fieldsResult)
		if !reflect.DeepEqual(fieldsOrig, fieldsResult) {
			return fmt.Errorf("unexpected fields for log entry %d\ngot\n%s\nwant\n%s", i, fieldsResult, fieldsOrig)
		}
	}
	return nil
}

// readLogRows reads log entries from mp.
//
// This function is for testing and debugging purposes only.
func (mp *inmemoryPart) readLogRows(sbu *stringsBlockUnmarshaler, vd *valuesDecoder) *LogRows {
	lr := GetLogRows(nil, nil)
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
			lr.MustAdd(streamID.tenantID, timestamp, tmp.rows[i])
			lr.streamIDs[len(lr.streamIDs)-1] = streamID
		}
		tmp.reset()
	}
	return lr
}
