package logstorage

import (
	"reflect"
	"testing"
)

func TestMarshalUnmarshalColumnNames(t *testing.T) {
	f := func(columnNames []string) {
		t.Helper()

		data := marshalColumnNames(nil, columnNames)
		result, err := unmarshalColumnNames(data)
		if err != nil {
			t.Fatalf("unexpected error when unmarshaling columnNames: %s", err)
		}
		if !reflect.DeepEqual(columnNames, result) {
			t.Fatalf("unexpected umarshaled columnNames\ngot\n%v\nwant\n%v", result, columnNames)
		}
	}

	f([]string{})

	f([]string{"", "foo", "bar"})

	f([]string{
		"asdf.sdf.dsfds.f fds. fds ",
		"foo",
		"bar.sdfsdf.fd",
		"",
		"aso apaa",
	})
}

func TestColumnNameIDGenerator(t *testing.T) {
	a := []string{"", "foo", "bar.baz", "asdf dsf dfs"}

	g := &columnNameIDGenerator{}

	for i, s := range a {
		id := g.getColumnNameID(s)
		if id != uint64(i) {
			t.Fatalf("first run: unexpected id generated for s=%q; got %d; want %d; g=%v", s, id, i, g)
		}
	}

	// Repeat the loop
	for i, s := range a {
		id := g.getColumnNameID(s)
		if id != uint64(i) {
			t.Fatalf("second run: unexpected id generated for s=%q; got %d; want %d; g=%v", s, id, i, g)
		}
	}
}
