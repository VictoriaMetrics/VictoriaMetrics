package logstorage

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestBlockMustInitFromRows(t *testing.T) {
	f := func(timestamps []int64, rows [][]Field, bExpected *block) {
		t.Helper()

		b := &block{}
		b.MustInitFromRows(timestamps, rows)
		if b.uncompressedSizeBytes() >= maxUncompressedBlockSize {
			t.Fatalf("expecting non-full block")
		}
		if !reflect.DeepEqual(b, bExpected) {
			t.Fatalf("unexpected block;\ngot\n%v\nwant\n%v", b, bExpected)
		}
		if n := b.Len(); n != len(timestamps) {
			t.Fatalf("unexpected block len; got %d; want %d", n, len(timestamps))
		}
		b.assertValid()
	}

	// An empty log entries
	f(nil, nil, &block{})
	f([]int64{}, [][]Field{}, &block{})

	// A single row
	timestamps := []int64{1234}
	rows := [][]Field{
		{
			{
				Name:  "msg",
				Value: "foo",
			},
			{
				Name:  "level",
				Value: "error",
			},
		},
	}
	bExpected := &block{
		timestamps: []int64{1234},
		constColumns: []Field{
			{
				Name:  "level",
				Value: "error",
			},
			{
				Name:  "msg",
				Value: "foo",
			},
		},
	}
	f(timestamps, rows, bExpected)

	// Multiple log entries with the same set of fields
	timestamps = []int64{3, 5}
	rows = [][]Field{
		{
			{
				Name:  "job",
				Value: "foo",
			},
			{
				Name:  "instance",
				Value: "host1",
			},
		},
		{
			{
				Name:  "job",
				Value: "foo",
			},
			{
				Name:  "instance",
				Value: "host2",
			},
		},
	}
	bExpected = &block{
		timestamps: []int64{3, 5},
		columns: []column{
			{
				name:   "instance",
				values: []string{"host1", "host2"},
			},
		},
		constColumns: []Field{
			{
				Name:  "job",
				Value: "foo",
			},
		},
	}
	f(timestamps, rows, bExpected)

	// Multiple log entries with distinct set of fields
	timestamps = []int64{3, 5, 10}
	rows = [][]Field{
		{
			{
				Name:  "msg",
				Value: "foo",
			},
			{
				Name:  "b",
				Value: "xyz",
			},
		},
		{
			{
				Name:  "b",
				Value: "xyz",
			},
			{
				Name:  "a",
				Value: "aaa",
			},
		},
		{
			{
				Name:  "b",
				Value: "xyz",
			},
		},
	}
	bExpected = &block{
		timestamps: []int64{3, 5, 10},
		columns: []column{
			{
				name:   "a",
				values: []string{"", "aaa", ""},
			},
			{
				name:   "msg",
				values: []string{"foo", "", ""},
			},
		},
		constColumns: []Field{
			{
				Name:  "b",
				Value: "xyz",
			},
		},
	}
	f(timestamps, rows, bExpected)
}

func TestBlockMustInitFromRowsFullBlock(t *testing.T) {
	const rowsCount = 2000
	timestamps := make([]int64, rowsCount)
	rows := make([][]Field, rowsCount)
	for i := range timestamps {
		fields := make([]Field, 10)
		for j := range fields {
			fields[j] = Field{
				Name:  fmt.Sprintf("field_%d", j),
				Value: "very very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong value",
			}
		}
		rows[i] = fields
	}

	b := getBlock()
	defer putBlock(b)
	b.MustInitFromRows(timestamps, rows)
	b.assertValid()
	if n := b.Len(); n != len(rows) {
		t.Fatalf("unexpected total log entries; got %d; want %d", n, len(rows))
	}
	if n := b.uncompressedSizeBytes(); n < maxUncompressedBlockSize {
		t.Fatalf("expecting full block with %d bytes; got %d bytes", maxUncompressedBlockSize, n)
	}
}

func TestBlockMustInitFromRows_Overflow(t *testing.T) {
	f := func(rowsCount int, fieldsPerRow int, expectedRowsProcessed int) {
		t.Helper()
		timestamps := make([]int64, rowsCount)
		rows := make([][]Field, rowsCount)
		for i := range timestamps {
			fields := make([]Field, fieldsPerRow)
			for j := range fields {
				fields[j] = Field{
					Name:  fmt.Sprintf("field_%d_%d", i, j),
					Value: "very very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong value",
				}
			}
			rows[i] = fields
		}
		b := getBlock()
		defer putBlock(b)
		b.MustInitFromRows(timestamps, rows)
		b.assertValid()
		if n := b.Len(); n != expectedRowsProcessed {
			t.Fatalf("unexpected total log entries; got %d; want %d", n, expectedRowsProcessed)
		}
	}
	f(10, 300, 6)
	f(10, 10, 10)
	f(15, 30, 15)
	f(maxColumnsPerBlock+1000, 1, maxColumnsPerBlock)
}

func TestBlockUncompressedSizeBytes(t *testing.T) {
	f := func(rows [][]Field) {
		t.Helper()

		// Build expected JSON and calculate actual serialized size
		var totalSize int
		for _, fields := range rows {
			m := make(map[string]string)
			m["_time"] = time.RFC3339Nano

			for _, f := range fields {
				if f.Value == "" {
					continue // skip empty values
				}
				key := getRawFieldName(f.Name)
				m[key] = f.Value
			}

			b, err := json.Marshal(m)
			if err != nil {
				t.Fatalf("failed to marshal JSON: %v", err)
			}
			totalSize += len(b) + 1 // +1 for newline if expected
		}

		b := &block{}
		timestamps := make([]int64, len(rows)) // values don't matter for size estimation
		b.MustInitFromRows(timestamps, rows)

		actualSize := b.uncompressedSizeBytes()
		if actualSize != totalSize {
			t.Fatalf("unexpected uncompressed size;\n got  %d\n want %d, testcase: %v", actualSize, totalSize, rows)
		}
	}

	// Empty block
	f(nil)

	// Single row with one field
	f([][]Field{
		{{"msg", "hello"}},
	})

	// Multiple rows with constant columns
	f([][]Field{
		{{"level", "info"}},
		{{"level", "info"}},
	})

	// Multiple rows with variable columns
	f([][]Field{
		{{"msg", "first"}},
		{{"msg", "second"}},
	})

	// Mixed constant and variable columns
	f([][]Field{
		{{"service", "api"}, {"msg", "start"}},
		{{"service", "api"}, {"msg", "end"}},
	})

	// Empty values ignored
	f([][]Field{
		{{"msg", "hello"}, {"empty", ""}},
		{{"msg", ""}, {"empty", "world"}},
	})
}

// TestEstimatedJSONRowLenMatchesBlockUncompressedSizeBytes verifies that
// EstimatedJSONRowLen and block.uncompressedSizeBytes stay in sync.
// If this test fails, update the calculations in both functions so that they
// produce identical results for the same set of log entries.
func TestEstimatedJSONRowLenMatchesBlockUncompressedSizeBytes(t *testing.T) {
	f := func(rows [][]Field) {
		t.Helper()

		b := &block{}
		timestamps := make([]int64, len(rows))
		b.MustInitFromRows(timestamps, rows)

		sizeBlock := b.uncompressedSizeBytes()
		sizeRows := 0
		for _, fields := range rows {
			sizeRows += EstimatedJSONRowLen(fields)
		}

		if sizeBlock != sizeRows {
			t.Fatalf("sizes mismatch: block=%d rows=%d for rows: %+v", sizeBlock, sizeRows, rows)
		}
	}

	// Test cases
	f(nil)

	// Single row with one field
	f([][]Field{{{"msg", "hello"}}})

	// Multiple rows with constant column
	f([][]Field{{{"level", "info"}}, {{"level", "info"}}})

	// Multiple rows with variable columns
	f([][]Field{{{"msg", "first"}}, {{"msg", "second"}}})

	// Mixed constant and variable columns
	f([][]Field{{{"service", "api"}, {"msg", "start"}}, {{"service", "api"}, {"msg", "end"}}})

	// Empty values ignored
	f([][]Field{{{"msg", "hello"}, {"empty", ""}}, {{"msg", ""}, {"empty", "world"}}})
}
