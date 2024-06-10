package logstorage

import (
	"fmt"
	"reflect"
	"testing"
)

func TestBlockMustInitFromRows(t *testing.T) {
	f := func(timestamps []int64, rows [][]Field, bExpected *block) {
		t.Helper()
		b := getBlock()
		defer putBlock(b)

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
