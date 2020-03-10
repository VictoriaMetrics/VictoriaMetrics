package csvimport

import (
	"reflect"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(format, s string) {
		t.Helper()
		cds, err := ParseColumnDescriptors(format)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", format, err)
		}
		var rs Rows
		rs.Unmarshal(s, cds)
		if len(rs.Rows) != 0 {
			t.Fatalf("unexpected rows unmarshaled: %#v", rs.Rows)
		}
	}
	// Invalid timestamp
	f("1:metric:foo,2:time:rfc3339", "234,foobar")

	// Missing columns
	f("3:metric:aaa", "123,456")
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(format, s string, rowsExpected []Row) {
		t.Helper()
		cds, err := ParseColumnDescriptors(format)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", format, err)
		}
		var rs Rows
		rs.Unmarshal(s, cds)
		if !reflect.DeepEqual(rs.Rows, rowsExpected) {
			t.Fatalf("unexpected rows;\ngot\n%v\nwant\n%v", rs.Rows, rowsExpected)
		}
		rs.Reset()

		// Unmarshal rows the second time
		rs.Unmarshal(s, cds)
		if !reflect.DeepEqual(rs.Rows, rowsExpected) {
			t.Fatalf("unexpected rows on the second unmarshal;\ngot\n%v\nwant\n%v", rs.Rows, rowsExpected)
		}
	}
	f("1:metric:foo", "", nil)
	f("1:metric:foo", `123`, []Row{
		{
			Metric: "foo",
			Value:  123,
		},
	})
	f("1:metric:foo,2:time:unix_s,3:label:foo,4:label:bar", `123,456,xxx,yy`, []Row{
		{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "foo",
					Value: "xxx",
				},
				{
					Key:   "bar",
					Value: "yy",
				},
			},
			Value:     123,
			Timestamp: 456000,
		},
	})

	// Multiple metrics
	f("2:metric:bar,1:metric:foo,3:label:foo,4:label:bar,5:time:custom:2006-01-02 15:04:05.999Z",
		`"2.34",5.6,"foo"",bar","aa",2015-08-10 20:04:40.123Z`, []Row{
			{
				Metric: "foo",
				Tags: []Tag{
					{
						Key:   "foo",
						Value: "foo\",bar",
					},
					{
						Key:   "bar",
						Value: "aa",
					},
				},
				Value:     2.34,
				Timestamp: 1439237080123,
			},
			{
				Metric: "bar",
				Tags: []Tag{
					{
						Key:   "foo",
						Value: "foo\",bar",
					},
					{
						Key:   "bar",
						Value: "aa",
					},
				},
				Value:     5.6,
				Timestamp: 1439237080123,
			},
		})
	f("2:label:symbol,3:time:custom:2006-01-02 15:04:05.999Z,4:metric:bid,5:metric:ask",
		`
"aaa","AUDCAD","2015-08-10 00:00:01.000Z",0.9725,0.97273
"aaa","AUDCAD","2015-08-10 00:00:02.000Z",0.97253,0.97276
`, []Row{
			{
				Metric: "bid",
				Tags: []Tag{
					{
						Key:   "symbol",
						Value: "AUDCAD",
					},
				},
				Value:     0.9725,
				Timestamp: 1439164801000,
			},
			{
				Metric: "ask",
				Tags: []Tag{
					{
						Key:   "symbol",
						Value: "AUDCAD",
					},
				},
				Value:     0.97273,
				Timestamp: 1439164801000,
			},
			{
				Metric: "bid",
				Tags: []Tag{
					{
						Key:   "symbol",
						Value: "AUDCAD",
					},
				},
				Value:     0.97253,
				Timestamp: 1439164802000,
			},
			{
				Metric: "ask",
				Tags: []Tag{
					{
						Key:   "symbol",
						Value: "AUDCAD",
					},
				},
				Value:     0.97276,
				Timestamp: 1439164802000,
			},
		})

	// Superflouos columns
	f("1:metric:foo", `123,456,foo,bar`, []Row{
		{
			Metric: "foo",
			Value:  123,
		},
	})
	f("2:metric:foo", `123,-45.6,foo,bar`, []Row{
		{
			Metric: "foo",
			Value:  -45.6,
		},
	})
}
