package logstorage

import (
	"reflect"
	"testing"
)

func TestPartHeaderReset(t *testing.T) {
	ph := &partHeader{
		CompressedSizeBytes:   123,
		UncompressedSizeBytes: 234,
		RowsCount:             1234,
		MinTimestamp:          3434,
		MaxTimestamp:          32434,
	}
	ph.reset()
	phZero := &partHeader{}
	if !reflect.DeepEqual(ph, phZero) {
		t.Fatalf("unexpected non-zero partHeader after reset: %v", ph)
	}
}
