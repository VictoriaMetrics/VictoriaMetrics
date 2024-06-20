package logstorage

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFilterStreamID(t *testing.T) {
	t.Parallel()

	// match
	ft := &filterStreamID{
		streamIDStr: "0000007b000001c8302bc96e02e54e5524b3a68ec271e55e",
	}
	testFilterMatchForStreamID(t, ft, []int{0, 3, 6, 9})

	ft = &filterStreamID{
		streamIDStr: "0000007b000001c850d9950ea6196b1a4812081265faa1c7",
	}
	testFilterMatchForStreamID(t, ft, []int{1, 4, 7})

	// mismatch
	ft = &filterStreamID{
		streamIDStr: "abc",
	}
	testFilterMatchForStreamID(t, ft, nil)
}

func testFilterMatchForStreamID(t *testing.T, f filter, expectedRowIdxs []int) {
	t.Helper()

	storagePath := t.Name()

	cfg := &StorageConfig{
		Retention: 100 * 365 * time.Duration(nsecsPerDay),
	}
	s := MustOpenStorage(storagePath, cfg)

	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}

	getMsgValue := func(i int) string {
		return fmt.Sprintf("some message value %d", i)
	}

	generateTestLogStreams(s, tenantID, getMsgValue, 10, 3)

	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = getMsgValue(idx)
		expectedTimestamps[i] = int64(idx * 100)
	}

	testFilterMatchForStorage(t, s, tenantID, f, "_msg", expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func generateTestLogStreams(s *Storage, tenantID TenantID, getMsgValue func(int) string, rowsCount, streamsCount int) {
	streamFields := []string{"host", "app"}
	lr := GetLogRows(streamFields, nil)
	var fields []Field
	for i := range rowsCount {
		fields = append(fields[:0], Field{
			Name:  "_msg",
			Value: getMsgValue(i),
		}, Field{
			Name:  "host",
			Value: fmt.Sprintf("host-%d", i%streamsCount),
		}, Field{
			Name:  "app",
			Value: "foobar",
		})
		timestamp := int64(i * 100)
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}
