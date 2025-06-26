package logstorage

import (
	"reflect"
	"testing"
)

func TestRenameField(t *testing.T) {
	f := func(fields []Field, oldNames []string, resultExpected string) {
		RenameField(fields, oldNames, "_msg")
		result := MarshalFieldsToJSON(nil, fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}

	f([]Field{
		{
			Name:  "message",
			Value: "test",
		},
		{
			Name:  "field.message",
			Value: "foo",
		},
	}, []string{"field.message", "message"}, `{"message":"test","_msg":"foo"}`)
}

func TestMarshalFieldsToJSON(t *testing.T) {
	f := func(fields []Field, resultExpected string) {
		t.Helper()

		result := MarshalFieldsToJSON(nil, fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}

	f(nil, "{}")
	f([]Field{}, "{}")

	f([]Field{
		{
			Name:  "foo",
			Value: "bar",
		},
	}, `{"foo":"bar"}`)

	f([]Field{
		{
			Name:  "foo\nbar",
			Value: "  \u001b[32m ",
		},
		{
			Name:  "  \u001b[11m ",
			Value: "АБв",
		},
	}, `{"foo\nbar":"  \u001b[32m ","  \u001b[11m ":"АБв"}`)
}

func TestMarshalFieldsToLogfmt(t *testing.T) {
	f := func(fields []Field, resultExpected string) {
		t.Helper()

		result := MarshalFieldsToLogfmt(nil, fields)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}

	f(nil, "")
	f([]Field{}, "")

	f([]Field{
		{
			Name:  "foo",
			Value: "bar",
		},
	}, `foo=bar`)

	f([]Field{
		{
			Name:  "foo",
			Value: "  \u001b[32m ",
		},
		{
			Name:  "bar",
			Value: "АБв",
		},
	}, `foo="  \u001b[32m " bar=АБв`)
}

func TestGetRowsSizeBytes(t *testing.T) {
	f := func(rows [][]Field, uncompressedSizeBytesExpected int) {
		t.Helper()
		sizeBytes := uncompressedRowsSizeBytes(rows)
		if sizeBytes != uint64(uncompressedSizeBytesExpected) {
			t.Fatalf("unexpected sizeBytes; got %d; want %d", sizeBytes, uncompressedSizeBytesExpected)
		}
	}
	f(nil, 0)
	f([][]Field{}, 0)
	f([][]Field{{}}, 48)
	f([][]Field{{{Name: "foo"}}}, 48)

	_, rows := newTestRows(1000, 10)
	f(rows, 286900)
}

func TestRowsAppendRows(t *testing.T) {
	var rs rows

	timestamps := []int64{1}
	rows := [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	rs.appendRows(timestamps, rows)
	if len(rs.timestamps) != 1 {
		t.Fatalf("unexpected number of row items; got %d; want 1", len(rs.timestamps))
	}
	rs.appendRows(timestamps, rows)
	if len(rs.timestamps) != 2 {
		t.Fatalf("unexpected number of row items; got %d; want 2", len(rs.timestamps))
	}
	for i := range rs.timestamps {
		if rs.timestamps[i] != timestamps[0] {
			t.Fatalf("unexpected timestamps copied; got %d; want %d", rs.timestamps[i], timestamps[0])
		}
		if !reflect.DeepEqual(rs.rows[i], rows[0]) {
			t.Fatalf("unexpected fields copied\ngot\n%v\nwant\n%v", rs.rows[i], rows[0])
		}
	}

	// append multiple log entries
	timestamps, rows = newTestRows(100, 4)
	rs.appendRows(timestamps, rows)
	if len(rs.timestamps) != 102 {
		t.Fatalf("unexpected number of row items; got %d; want 102", len(rs.timestamps))
	}
	for i := range timestamps {
		if rs.timestamps[i+2] != timestamps[i] {
			t.Fatalf("unexpected timestamps copied; got %d; want %d", rs.timestamps[i+2], timestamps[i])
		}
		if !reflect.DeepEqual(rs.rows[i+2], rows[i]) {
			t.Fatalf("unexpected log entry copied\ngot\n%v\nwant\n%v", rs.rows[i+2], rows[i])
		}
	}

	// reset rows
	rs.reset()
	if len(rs.timestamps) != 0 {
		t.Fatalf("unexpected non-zero number of row items after reset: %d", len(rs.timestamps))
	}
}

func TestMergeRows(t *testing.T) {
	f := func(timestampsA, timestampsB []int64, fieldsA, fieldsB [][]Field, timestampsExpected []int64, rowsExpected [][]Field) {
		t.Helper()
		var rs rows
		rs.mergeRows(timestampsA, timestampsB, fieldsA, fieldsB)
		if !reflect.DeepEqual(rs.timestamps, timestampsExpected) {
			t.Fatalf("unexpected timestamps after merge\ngot\n%v\nwant\n%v", rs.timestamps, timestampsExpected)
		}
		if !reflect.DeepEqual(rs.rows, rowsExpected) {
			t.Fatalf("unexpected rows after merge\ngot\n%v\nwant\n%v", rs.rows, rowsExpected)
		}

		// check that the result doesn't change when merging in reverse order
		rs.reset()
		rs.mergeRows(timestampsB, timestampsA, fieldsB, fieldsA)
		if !reflect.DeepEqual(rs.timestamps, timestampsExpected) {
			t.Fatalf("unexpected timestamps after reverse merge\ngot\n%v\nwant\n%v", rs.timestamps, timestampsExpected)
		}
		if !reflect.DeepEqual(rs.rows, rowsExpected) {
			t.Fatalf("unexpected rows after reverse merge\ngot\n%v\nwant\n%v", rs.rows, rowsExpected)
		}
	}

	f(nil, nil, nil, nil, nil, nil)

	// merge single entry with zero entries
	timestampsA := []int64{123}
	timestampsB := []int64{}

	fieldsA := [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	fieldsB := [][]Field{}

	resultTimestamps := []int64{123}
	resultFields := [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	f(timestampsA, timestampsB, fieldsA, fieldsB, resultTimestamps, resultFields)

	// merge two single entries
	timestampsA = []int64{123}
	timestampsB = []int64{43323}

	fieldsA = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}
	fieldsB = [][]Field{
		{
			{
				Name:  "asdfds",
				Value: "asdfsa",
			},
		},
	}

	resultTimestamps = []int64{123, 43323}
	resultFields = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "asdfds",
				Value: "asdfsa",
			},
		},
	}
	f(timestampsA, timestampsB, fieldsA, fieldsB, resultTimestamps, resultFields)

	// merge identical entries
	timestampsA = []int64{123, 456}
	timestampsB = []int64{123, 456}

	fieldsA = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "foo",
				Value: "baz",
			},
		},
	}
	fieldsB = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "foo",
				Value: "baz",
			},
		},
	}

	resultTimestamps = []int64{123, 123, 456, 456}
	resultFields = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "foo",
				Value: "baz",
			},
		},
		{
			{
				Name:  "foo",
				Value: "baz",
			},
		},
	}
	f(timestampsA, timestampsB, fieldsA, fieldsB, resultTimestamps, resultFields)

	// merge interleaved entries
	timestampsA = []int64{12, 13432}
	timestampsB = []int64{3, 43323}

	fieldsA = [][]Field{
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "xfoo",
				Value: "xbar",
			},
		},
	}
	fieldsB = [][]Field{
		{
			{
				Name:  "asd",
				Value: "assa",
			},
		},
		{
			{
				Name:  "asdfds",
				Value: "asdfsa",
			},
		},
	}

	resultTimestamps = []int64{3, 12, 13432, 43323}
	resultFields = [][]Field{
		{
			{
				Name:  "asd",
				Value: "assa",
			},
		},
		{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
		{
			{
				Name:  "xfoo",
				Value: "xbar",
			},
		},
		{
			{
				Name:  "asdfds",
				Value: "asdfsa",
			},
		},
	}
	f(timestampsA, timestampsB, fieldsA, fieldsB, resultTimestamps, resultFields)
}
