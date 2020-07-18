package prometheus

import (
	"bytes"
	"compress/gzip"
	"reflect"
	"testing"
)

func TestParseStream(t *testing.T) {
	f := func(s string, rowsExpected []Row) {
		t.Helper()
		bb := bytes.NewBufferString(s)
		var result []Row
		err := ParseStream(bb, false, func(rows []Row) error {
			result = appendRowCopies(result, rows)
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if !reflect.DeepEqual(result, rowsExpected) {
			t.Fatalf("unexpected rows parsed; got\n%v\nwant\n%v", result, rowsExpected)
		}

		// Parse compressed stream.
		bb.Reset()
		zw := gzip.NewWriter(bb)
		if _, err := zw.Write([]byte(s)); err != nil {
			t.Fatalf("unexpected error when gzipping %q: %s", s, err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("unexpected error when closing gzip writer: %s", err)
		}
		result = nil
		err = ParseStream(bb, true, func(rows []Row) error {
			result = appendRowCopies(result, rows)
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error when parsing compressed %q: %s", s, err)
		}
		if !reflect.DeepEqual(result, rowsExpected) {
			t.Fatalf("unexpected rows parsed; got\n%v\nwant\n%v", result, rowsExpected)
		}
	}

	f("", nil)
	f("foo 123 456", []Row{{
		Metric:    "foo",
		Value:     123,
		Timestamp: 456,
	}})
	f(`foo{bar="baz"} 1 2`+"\n"+`aaa{} 3 4`, []Row{
		{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "baz",
			}},
			Value:     1,
			Timestamp: 2,
		},
		{
			Metric:    "aaa",
			Value:     3,
			Timestamp: 4,
		},
	})
}

func appendRowCopies(dst, src []Row) []Row {
	for _, r := range src {
		// Make a copy of r, since r may contain garbage after returning from the callback to ParseStream.
		var rCopy Row
		rCopy.Metric = copyString(r.Metric)
		rCopy.Value = r.Value
		rCopy.Timestamp = r.Timestamp
		for _, tag := range r.Tags {
			rCopy.Tags = append(rCopy.Tags, Tag{
				Key:   copyString(tag.Key),
				Value: copyString(tag.Value),
			})
		}
		dst = append(dst, rCopy)
	}
	return dst
}

func copyString(s string) string {
	return string(append([]byte(nil), s...))
}
