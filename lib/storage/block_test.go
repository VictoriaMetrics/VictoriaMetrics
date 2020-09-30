package storage

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestBlockMarshalUnmarshalPortable(t *testing.T) {
	var b Block
	for i := 0; i < 1000; i++ {
		b.Reset()
		rowsCount := rand.Intn(maxRowsPerBlock) + 1
		b.timestamps = getRandTimestamps(rowsCount)
		b.values = getRandValues(rowsCount)
		b.bh.Scale = int16(rand.Intn(30) - 15)
		b.bh.PrecisionBits = 64
		testBlockMarshalUnmarshalPortable(t, &b)
	}
}

func testBlockMarshalUnmarshalPortable(t *testing.T, b *Block) {
	var b1, b2 Block
	b1.CopyFrom(b)
	rowsCount := len(b.values)
	data := b1.MarshalPortable(nil)
	if b1.bh.RowsCount != uint32(rowsCount) {
		t.Fatalf("unexpected number of rows marshaled; got %d; want %d", b1.bh.RowsCount, rowsCount)
	}
	tail, err := b2.UnmarshalPortable(data)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(tail) > 0 {
		t.Fatalf("unexpected non-empty tail: %X", tail)
	}
	compareBlocksPortable(t, &b2, b, &b1.bh)

	// Verify non-empty prefix and suffix
	prefix := "prefix"
	suffix := "suffix"
	data = append(data[:0], prefix...)
	data = b1.MarshalPortable(data)
	if b1.bh.RowsCount != uint32(rowsCount) {
		t.Fatalf("unexpected number of rows marshaled; got %d; want %d", b1.bh.RowsCount, rowsCount)
	}
	if !strings.HasPrefix(string(data), prefix) {
		t.Fatalf("unexpected prefix in %X; want %X", data, prefix)
	}
	data = data[len(prefix):]
	data = append(data, suffix...)
	tail, err = b2.UnmarshalPortable(data)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(tail) != suffix {
		t.Fatalf("unexpected tail; got %X; want %X", tail, suffix)
	}
	compareBlocksPortable(t, &b2, b, &b1.bh)
}

func compareBlocksPortable(t *testing.T, b1, b2 *Block, bhExpected *blockHeader) {
	t.Helper()
	if b1.bh.MinTimestamp != bhExpected.MinTimestamp {
		t.Fatalf("unexpected MinTimestamp; got %d; want %d", b1.bh.MinTimestamp, bhExpected.MinTimestamp)
	}
	if b1.bh.FirstValue != bhExpected.FirstValue {
		t.Fatalf("unexpected FirstValue; got %d; want %d", b1.bh.FirstValue, bhExpected.FirstValue)
	}
	if b1.bh.RowsCount != bhExpected.RowsCount {
		t.Fatalf("unexpected RowsCount; got %d; want %d", b1.bh.RowsCount, bhExpected.RowsCount)
	}
	if b1.bh.Scale != bhExpected.Scale {
		t.Fatalf("unexpected Scale; got %d; want %d", b1.bh.Scale, bhExpected.Scale)
	}
	if b1.bh.TimestampsMarshalType != bhExpected.TimestampsMarshalType {
		t.Fatalf("unexpected TimestampsMarshalType; got %d; want %d", b1.bh.TimestampsMarshalType, bhExpected.TimestampsMarshalType)
	}
	if b1.bh.ValuesMarshalType != bhExpected.ValuesMarshalType {
		t.Fatalf("unexpected ValuesMarshalType; got %d; want %d", b1.bh.ValuesMarshalType, bhExpected.ValuesMarshalType)
	}
	if b1.bh.PrecisionBits != bhExpected.PrecisionBits {
		t.Fatalf("unexpected PrecisionBits; got %d; want %d", b1.bh.PrecisionBits, bhExpected.PrecisionBits)
	}
	if !reflect.DeepEqual(b1.values, b2.values) {
		t.Fatalf("unexpected values; got %d; want %d", b1.values, b2.values)
	}
	if !reflect.DeepEqual(b1.timestamps, b2.timestamps) {
		t.Fatalf("unexpected timestamps; got %d; want %d", b1.timestamps, b2.timestamps)
	}
	if len(b1.values) != int(bhExpected.RowsCount) {
		t.Fatalf("unexpected number of values; got %d; want %d", len(b1.values), bhExpected.RowsCount)
	}
	if len(b1.timestamps) != int(bhExpected.RowsCount) {
		t.Fatalf("unexpected number of timestamps; got %d; want %d", len(b1.timestamps), bhExpected.RowsCount)
	}
}

func getRandValues(rowsCount int) []int64 {
	a := make([]int64, rowsCount)
	for i := 0; i < rowsCount; i++ {
		a[i] = int64(rand.Intn(1e5) - 0.5e5)
	}
	return a
}

func getRandTimestamps(rowsCount int) []int64 {
	a := make([]int64, rowsCount)
	ts := int64(rand.Intn(1e9))
	for i := 0; i < rowsCount; i++ {
		a[i] = ts
		ts += int64(rand.Intn(1e5))
	}
	return a
}
