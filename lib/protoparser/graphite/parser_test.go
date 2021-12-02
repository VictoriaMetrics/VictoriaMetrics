package graphite

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

func TestUnmarshalMetricAndTagsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var r Row
		_, err := r.UnmarshalMetricAndTags(s, nil)
		if err == nil {
			t.Fatalf("expecting non-nil error for UnmarshalMetricAndTags(%q)", s)
		}
	}
	f("")
	f(";foo=bar")
	f(" ")
	f("foo ;bar=baz")
	f("f oo;bar=baz")
	f("foo;bar=baz   ")
	f("foo;bar= baz")
	f("foo;bar=b az")
	f("foo;b ar=baz")
}

func TestUnmarshalMetricAndTagsSuccess(t *testing.T) {
	f := func(s string, rExpected *Row) {
		t.Helper()
		var r Row
		_, err := r.UnmarshalMetricAndTags(s, nil)
		if err != nil {
			t.Fatalf("unexpected error in UnmarshalMetricAndTags(%q): %s", s, err)
		}
		if !reflect.DeepEqual(&r, rExpected) {
			t.Fatalf("unexpected row;\ngot\n%+v\nwant\n%+v", &r, rExpected)
		}
	}
	f("foo", &Row{
		Metric: "foo",
	})
	f("foo;bar=123;baz=aabb", &Row{
		Metric: "foo",
		Tags: []Tag{
			{
				Key:   "bar",
				Value: "123",
			},
			{
				Key:   "baz",
				Value: "aabb",
			},
		},
	})
}

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}

		// Try again
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}
	}

	// Missing metric
	f(" 123 455")

	// Missing value
	f("aaa")

	// unexpected space in tag value
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/99
	f("s;tag1=aaa1;tag2=bb b2;tag3=ccc3 1")

	// invalid value
	f("aa bb")

	// invalid timestamp
	f("aa 123 bar")
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(s)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
	}

	// Empty line
	f("", &Rows{})
	f("\r", &Rows{})
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})

	// Single line
	f("foobar -123.456 789", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
		}},
	})
	f("foo.bar 123.456 789\n", &Rows{
		Rows: []Row{{
			Metric:    "foo.bar",
			Value:     123.456,
			Timestamp: 789,
		}},
	})

	// Missing timestamp
	f("aaa 1123", &Rows{
		Rows: []Row{{
			Metric: "aaa",
			Value:  1123,
		}},
	})
	f("aaa 1123 -1", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: -1,
		}},
	})

	// Timestamp bigger than 1<<31
	f("aaa 1123 429496729600", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 429496729600,
		}},
	})

	// Floating-point timestamp
	// See https://github.com/graphite-project/carbon/blob/b0ba62a62d40a37950fed47a8f6ae6d0f02e6af5/lib/carbon/protocols.py#L197
	f("aaa 1123 4294.943", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 4294,
		}},
	})

	// Tags
	f("foo;bar=baz 1 2", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "baz",
			}},
			Value:     1,
			Timestamp: 2,
		}},
	})
	// Empty tags
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1100
	f("foo; 1", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags:   []Tag{},
			Value:  1,
		}},
	})
	f("foo; 1 2", &Rows{
		Rows: []Row{{
			Metric:    "foo",
			Tags:      []Tag{},
			Value:     1,
			Timestamp: 2,
		}},
	})
	// Empty tag name or value
	f("foo;bar 1 2", &Rows{
		Rows: []Row{{
			Metric:    "foo",
			Tags:      []Tag{},
			Value:     1,
			Timestamp: 2,
		}},
	})
	f("foo;bar=baz;aa=;x=y;=z 1 2", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "bar",
					Value: "baz",
				},
				{
					Key:   "x",
					Value: "y",
				},
			},
			Value:     1,
			Timestamp: 2,
		}},
	})

	// Multi lines
	f("foo 0.3 2\naaa 3\nbar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2,
			},
			{
				Metric: "aaa",
				Value:  3,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43,
			},
		},
	})

	// Multi lines with invalid line
	f("foo 0.3 2\naaa\nbar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43,
			},
		},
	})

	// With tab as separator
	// See https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528
	f("foo.baz\t125.456\t1789\n", &Rows{
		Rows: []Row{{
			Metric:    "foo.baz",
			Value:     125.456,
			Timestamp: 1789,
		}},
	})
	// With tab as separator and tags
	f("foo;baz=bar;bb=;y=x;=z\t1\t2", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "baz",
					Value: "bar",
				},
				{
					Key:   "y",
					Value: "x",
				},
			},
			Value:     1,
			Timestamp: 2,
		}},
	})

	// Whitespace after the timestamp
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1865
	f("foo.baz 125 1789 \na 1.34 567\t  ", &Rows{
		Rows: []Row{
			{
				Metric:    "foo.baz",
				Value:     125,
				Timestamp: 1789,
			},
			{
				Metric:    "a",
				Value:     1.34,
				Timestamp: 567,
			},
		},
	})

	// Multiple whitespaces as separators
	f("foo.baz \t125  1789 \t\n", &Rows{
		Rows: []Row{{
			Metric:    "foo.baz",
			Value:     125,
			Timestamp: 1789,
		}},
	})
}

func Test_streamContext_Read(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		ctx := getStreamContext(strings.NewReader(s))
		if !ctx.Read() {
			t.Fatalf("expecting successful read")
		}
		uw := getUnmarshalWork()
		callbackCalls := 0
		uw.callback = func(rows []Row) {
			callbackCalls++
			if len(rows) != len(rowsExpected.Rows) {
				t.Fatalf("different len of expected rows;\ngot\n%+v;\nwant\n%+v", rows, rowsExpected.Rows)
			}
			if !reflect.DeepEqual(rows, rowsExpected.Rows) {
				t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows, rowsExpected.Rows)
			}
		}
		uw.reqBuf = append(uw.reqBuf[:0], ctx.reqBuf...)
		uw.Unmarshal()
		if callbackCalls != 1 {
			t.Fatalf("unexpected number of callback calls; got %d; want 1", callbackCalls)
		}
	}

	// Full line without tags
	f("aaa 1123 345", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 345 * 1000,
		}},
	})
	// Full line with tags
	f("aaa;x=y 1123 345", &Rows{
		Rows: []Row{{
			Metric: "aaa",
			Tags: []Tag{{
				Key:   "x",
				Value: "y",
			}},
			Value:     1123,
			Timestamp: 345 * 1000,
		}},
	})
	// missing timestamp.
	// Note that this test may be flaky due to timing issues. TODO: fix it
	f("aaa 1123", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
	// -1 timestamp. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/610
	// Note that this test may be flaky due to timing issues. TODO: fix it.
	f("aaa 1123 -1", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: int64(fasttime.UnixTimestamp()) * 1000,
		}},
	})
}
