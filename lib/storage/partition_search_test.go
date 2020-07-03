package storage

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

func TestPartitionSearch(t *testing.T) {
	ptt := timestampFromTime(time.Now())
	var ptr TimeRange
	ptr.fromPartitionTimestamp(ptt)

	t.Run("SinglePart", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 1, 10000, 10)
	})

	t.Run("SingleRowPerPart", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 1000, 1, 10)
	})

	t.Run("SingleTSID", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 10000, 1)
	})

	t.Run("ManyParts", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 300, 300, 20)
	})

	t.Run("ManyTSIDs", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 10000, 1000)
	})

	t.Run("ExactTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp,
			MaxTimestamp: ptr.MaxTimestamp,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("InnerTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("OuterTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp - 1e6,
			MaxTimestamp: ptr.MaxTimestamp + 1e6,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("LowTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp - 2e6,
			MaxTimestamp: ptr.MinTimestamp - 1e6,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("HighTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MaxTimestamp + 1e6,
			MaxTimestamp: ptr.MaxTimestamp + 2e6,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("LowerEndTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp - 1e6,
			MaxTimestamp: ptr.MaxTimestamp - 4e3,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})

	t.Run("HigherEndTimeRange", func(t *testing.T) {
		tr := TimeRange{
			MinTimestamp: ptr.MinTimestamp + 4e3,
			MaxTimestamp: ptr.MaxTimestamp + 1e6,
		}
		testPartitionSearchEx(t, ptt, tr, 5, 1000, 10)
	})
}

func testPartitionSearchEx(t *testing.T, ptt int64, tr TimeRange, partsCount, maxRowsPerPart, tsidsCount int) {
	t.Helper()

	// Generate tsids to search.
	var tsids []TSID
	var tsid TSID
	for i := 0; i < 25; i++ {
		tsid.MetricID = uint64(rand.Intn(tsidsCount * 2))
		tsids = append(tsids, tsid)
	}
	sort.Slice(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) })

	// Generate the expected blocks.

	rowsCountExpected := int64(0)
	rbsExpected := []rawBlock{}
	var ptr TimeRange
	ptr.fromPartitionTimestamp(ptt)
	var rowss [][]rawRow
	for i := 0; i < partsCount; i++ {
		var rows []rawRow
		var r rawRow
		r.PrecisionBits = 30
		timestamp := ptr.MinTimestamp
		rowsCount := 1 + rand.Intn(maxRowsPerPart)
		for j := 0; j < rowsCount; j++ {
			r.TSID.MetricID = uint64(rand.Intn(tsidsCount))
			r.Timestamp = timestamp
			r.Value = float64(int(rand.NormFloat64() * 1e5))

			timestamp += int64(rand.Intn(1e4))
			if timestamp > ptr.MaxTimestamp {
				break
			}

			rows = append(rows, r)
			rowsCountExpected++
		}
		rbs := getTestExpectedRawBlocks(rows, tsids, tr)
		rbsExpected = append(rbsExpected, rbs...)
		rowss = append(rowss, rows)
	}
	sort.Slice(rbsExpected, func(i, j int) bool {
		a, b := rbsExpected[i], rbsExpected[j]
		if a.TSID.Less(&b.TSID) {
			return true
		}
		if b.TSID.Less(&a.TSID) {
			return false
		}
		return a.Timestamps[0] < b.Timestamps[0]
	})

	// Create partition from rowss and test search on it.
	pt, err := createPartition(ptt, "./small-table", "./big-table", nilGetDeletedMetricIDs)
	if err != nil {
		t.Fatalf("cannot create partition: %s", err)
	}
	smallPartsPath := pt.smallPartsPath
	bigPartsPath := pt.bigPartsPath
	defer func() {
		if err := os.RemoveAll("./small-table"); err != nil {
			t.Fatalf("cannot remove small parts directory: %s", err)
		}
		if err := os.RemoveAll("./big-table"); err != nil {
			t.Fatalf("cannot remove big parts directory: %s", err)
		}
	}()
	for _, rows := range rowss {
		pt.AddRows(rows)

		// Flush just added rows to a separate partition.
		pt.flushRawRows(true)
	}
	testPartitionSearch(t, pt, tsids, tr, rbsExpected, -1)
	pt.MustClose()

	// Open the created partition and test search on it.
	pt, err = openPartition(smallPartsPath, bigPartsPath, nilGetDeletedMetricIDs)
	if err != nil {
		t.Fatalf("cannot open partition: %s", err)
	}
	testPartitionSearch(t, pt, tsids, tr, rbsExpected, rowsCountExpected)
	pt.MustClose()
}

func testPartitionSearch(t *testing.T, pt *partition, tsids []TSID, tr TimeRange, rbsExpected []rawBlock, rowsCountExpected int64) {
	t.Helper()

	if err := testPartitionSearchSerial(pt, tsids, tr, rbsExpected, rowsCountExpected); err != nil {
		t.Fatalf("unexpected error in serial partition search: %s", err)
	}

	ch := make(chan error, 5)
	for i := 0; i < cap(ch); i++ {
		go func() {
			ch <- testPartitionSearchSerial(pt, tsids, tr, rbsExpected, rowsCountExpected)
		}()
	}
	for i := 0; i < cap(ch); i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error in concurrent partition search: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout in concurrent partition search")
		}
	}
}

func testPartitionSearchSerial(pt *partition, tsids []TSID, tr TimeRange, rbsExpected []rawBlock, rowsCountExpected int64) error {
	if rowsCountExpected >= 0 {
		// Verify rows count only on partition opened from files.
		//
		// Online created partition may contain incomplete number of rows
		// due to the race with raw rows flusher.
		var m partitionMetrics
		pt.UpdateMetrics(&m)
		rowsCount := m.BigRowsCount + m.SmallRowsCount
		if rowsCount != uint64(rowsCountExpected) {
			return fmt.Errorf("unexpected rows count; got %d; want %d", rowsCount, rowsCountExpected)
		}
	}

	bs := []Block{}
	var pts partitionSearch
	pts.Init(pt, tsids, tr)
	for pts.NextBlock() {
		var b Block
		pts.BlockRef.MustReadBlock(&b, true)
		bs = append(bs, b)
	}
	if err := pts.Error(); err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}
	pts.MustClose()
	rbs := newTestRawBlocks(bs, tr)
	if err := testEqualRawBlocks(rbs, rbsExpected); err != nil {
		return fmt.Errorf("unequal blocks: %w", err)
	}

	if rowsCountExpected >= 0 {
		var m partitionMetrics
		pt.UpdateMetrics(&m)
		rowsCount := m.BigRowsCount + m.SmallRowsCount
		if rowsCount != uint64(rowsCountExpected) {
			return fmt.Errorf("unexpected rows count after search; got %d; want %d", rowsCount, rowsCountExpected)
		}
	}

	// verify that empty tsids returns empty result
	pts.Init(pt, []TSID{}, tr)
	if pts.NextBlock() {
		return fmt.Errorf("unexpected block got for an empty tsids list: %+v", pts.BlockRef)
	}
	if err := pts.Error(); err != nil {
		return fmt.Errorf("unexpected error on empty tsids list: %w", err)
	}
	pts.MustClose()

	return nil
}

func nilGetDeletedMetricIDs() *uint64set.Set {
	return nil
}
