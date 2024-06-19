package logstorage

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFilterTime(t *testing.T) {
	t.Parallel()

	timestamps := []int64{
		1,
		9,
		123,
		456,
		789,
	}

	// match
	ft := &filterTime{
		minTimestamp: -10,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterTime{
		minTimestamp: -10,
		maxTimestamp: 10,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0, 1})

	ft = &filterTime{
		minTimestamp: 1,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{0})

	ft = &filterTime{
		minTimestamp: 2,
		maxTimestamp: 456,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterTime{
		minTimestamp: 2,
		maxTimestamp: 457,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{1, 2, 3})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 788,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 789,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterTime{
		minTimestamp: 120,
		maxTimestamp: 10000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{2, 3, 4})

	ft = &filterTime{
		minTimestamp: 789,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, []int{4})

	// mismatch
	ft = &filterTime{
		minTimestamp: -1000,
		maxTimestamp: 0,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)

	ft = &filterTime{
		minTimestamp: 790,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, ft, nil)
}

func testFilterMatchForTimestamps(t *testing.T, timestamps []int64, f filter, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	storagePath := t.Name()
	cfg := &StorageConfig{
		Retention: 100 * 365 * time.Duration(nsecsPerDay),
	}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	getValue := func(rowIdx int) string {
		return fmt.Sprintf("some value for row %d", rowIdx)
	}
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromTimestamps(s, tenantID, timestamps, getValue)

	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = getValue(idx)
		expectedTimestamps[i] = timestamps[idx]
	}

	testFilterMatchForStorage(t, s, tenantID, f, "_msg", expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func generateRowsFromTimestamps(s *Storage, tenantID TenantID, timestamps []int64, getValue func(rowIdx int) string) {
	lr := GetLogRows(nil, nil)
	var fields []Field
	for i, timestamp := range timestamps {
		fields = append(fields[:0], Field{
			Name:  "_msg",
			Value: getValue(i),
		})
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}
