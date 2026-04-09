package csvimport

import (
	"reflect"
	"strings"
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
	f("1:metric:foo,2:time:unix_s", "234,foobar")
	f("1:metric:foo,2:time:unix_ms", "234,foobar")
	f("1:metric:foo,2:time:unix_ns", "234,foobar")
	f("1:metric:foo,2:time:custom:foobar", "234,234")

	// Too big timestamp in seconds.
	f("1:metric:foo,2:time:unix_s", "1,12345678901234567")

	// Missing columns
	f("3:metric:aaa", "123,456")
	f("1:metric:foo,2:label:bar", "123")
	f("1:label:foo,2:metric:bar", "aaa")

	// Invalid value
	f("1:metric:foo", "12foobar")
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

	// Superfluous columns
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
	// skip metrics with empty values
	f("1:metric:foo,2:metric:bar,3:metric:baz,4:metric:quux", `1,,,2`, []Row{
		{
			Metric: "foo",
			Value:  1,
		},
		{
			Metric: "quux",
			Value:  2,
		},
	})
	// last metric with empty value
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4048
	f("1:metric:foo,2:metric:bar", `123,`, []Row{
		{
			Metric: "foo",
			Value:  123,
		},
	})
	// all the metrics with empty values
	f(`1:metric:foo,2:metric:bar,3:label:xx`, `,,abc`, nil)
	// labels with empty value
	f("1:metric:foo,2:label:bar,3:label:baz,4:label:xxx", "123,x,,", []Row{
		{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "bar",
					Value: "x",
				},
			},
			Value: 123,
		},
	})
	f("1:metric:foo,2:label:bar,3:label:baz,4:label:xxx", "123,,,", []Row{
		{
			Metric: "foo",
			Value:  123,
		},
	})
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3540
	f("1:label:mytest,2:time:rfc3339,3:metric:M10,4:metric:M20,5:metric:M30,6:metric:M40,7:metric:M50,8:metric:M60",
		`test,2022-12-25T16:57:12+01:00,10,20,30,,,60,70,80`, []Row{
			{
				Metric: "M10",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     10,
			},
			{
				Metric: "M20",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     20,
			},
			{
				Metric: "M30",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     30,
			},
			{
				Metric: "M60",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     60,
			},
		})
	// rfc3339 with millisecond precision
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5837
	f("1:label:mytest,2:time:rfc3339,3:metric:M10,4:metric:M20,5:metric:M30,6:metric:M40,7:metric:M50,8:metric:M60",
		`test,2022-12-25T16:57:12.000+01:00,10,20,30,,,60,70,80`, []Row{
			{
				Metric: "M10",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     10,
			},
			{
				Metric: "M20",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     20,
			},
			{
				Metric: "M30",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     30,
			},
			{
				Metric: "M60",
				Tags: []Tag{
					{
						Key:   "mytest",
						Value: "test",
					},
				},
				Timestamp: 1671983832000,
				Value:     60,
			},
		})
}

func TestHeaderDetection(t *testing.T) {
	f := func(format, s string, rowsExpected []Row) {
		t.Helper()
		cds, err := ParseColumnDescriptors(format)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", format, err)
		}
		var rs Rows
		rs.UnmarshalDetectHeader(s, cds)
		if !reflect.DeepEqual(rs.Rows, rowsExpected) {
			t.Fatalf("unexpected rows;\ngot\n%v\nwant\n%v", rs.Rows, rowsExpected)
		}
		rs.Reset()
		rs.UnmarshalDetectHeader(s, cds)
		if !reflect.DeepEqual(rs.Rows, rowsExpected) {
			t.Fatalf("unexpected rows on second unmarshal;\ngot\n%v\nwant\n%v", rs.Rows, rowsExpected)
		}
	}

	// non-numeric metric column
	f("1:metric:foo", "value\n123", []Row{
		{Metric: "foo", Value: 123},
	})
	f("1:metric:foo", "foo\n42", []Row{
		{Metric: "foo", Value: 42},
	})

	// non-numeric timestamp column
	f("1:metric:foo,2:time:unix_s", "value,timestamp\n123,456", []Row{
		{Metric: "foo", Value: 123, Timestamp: 456000},
	})
	f("1:metric:foo,2:time:unix_ms", "value,timestamp\n10,2000", []Row{
		{Metric: "foo", Value: 10, Timestamp: 2000},
	})
	f("1:metric:foo,2:time:rfc3339", "value,timestamp\n10,2024-01-01T00:00:00Z", []Row{
		{Metric: "foo", Value: 10, Timestamp: 1704067200000},
	})

	// header with labels
	f("1:label:host,2:metric:cpu,3:time:unix_s",
		"host,value,timestamp\nmyhost,99.5,1000",
		[]Row{
			{Metric: "cpu", Tags: []Tag{{Key: "host", Value: "myhost"}}, Value: 99.5, Timestamp: 1000000},
		})

	// header with multiple data rows
	f("1:metric:foo,2:time:unix_s",
		"value,timestamp\n10,100\n20,200\n30,300",
		[]Row{
			{Metric: "foo", Value: 10, Timestamp: 100000},
			{Metric: "foo", Value: 20, Timestamp: 200000},
			{Metric: "foo", Value: 30, Timestamp: 300000},
		})

	// header with multiple metrics per row
	f("1:metric:bid,2:metric:ask,3:time:unix_s",
		"bid,ask,timestamp\n1.5,1.6,1000",
		[]Row{
			{Metric: "bid", Value: 1.5, Timestamp: 1000000},
			{Metric: "ask", Value: 1.6, Timestamp: 1000000},
		})

	// one non-numeric metric column is enough to detect the header
	f("1:metric:foo,2:metric:bar", "123,count\n1,2", []Row{
		{Metric: "foo", Value: 1},
		{Metric: "bar", Value: 2},
	})

	// header only, no data
	f("1:metric:foo,2:time:unix_s", "value,timestamp", nil)

	// column gap
	f("3:metric:foo", "a,b,value\na,b,123", []Row{
		{Metric: "foo", Value: 123},
	})

	// numeric first row
	f("1:metric:foo,2:time:unix_s", "123,456", []Row{
		{Metric: "foo", Value: 123, Timestamp: 456000},
	})
	f("1:metric:foo", "123\n456", []Row{
		{Metric: "foo", Value: 123},
		{Metric: "foo", Value: 456},
	})

	// valid rfc3339 parses as data, not header
	f("1:metric:foo,2:time:rfc3339", "123,2024-01-01T00:00:00Z", []Row{
		{Metric: "foo", Value: 123, Timestamp: 1704067200000},
	})

	// No header — text label columns don't trigger detection
	f("1:label:host,2:metric:foo,3:time:unix_s",
		"myhost,42,1000\notherhost,99,2000",
		[]Row{
			{Metric: "foo", Tags: []Tag{{Key: "host", Value: "myhost"}}, Value: 42, Timestamp: 1000000},
			{Metric: "foo", Tags: []Tag{{Key: "host", Value: "otherhost"}}, Value: 99, Timestamp: 2000000},
		})

	// numeric label "404" is not a false positive
	f("1:label:status,2:metric:count,3:time:unix_s",
		"404,100,1704067200",
		[]Row{
			{Metric: "count", Tags: []Tag{{Key: "status", Value: "404"}}, Value: 100, Timestamp: 1704067200000},
		})

	// empty input
	f("1:metric:foo", "", nil)

	// single numeric row
	f("1:metric:foo", "69", []Row{
		{Metric: "foo", Value: 69},
	})
}

func TestUnmarshalBackwardCompatibility(t *testing.T) {
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
	}

	f("1:metric:foo,2:time:unix_s", "123,456", []Row{
		{Metric: "foo", Value: 123, Timestamp: 456000},
	})
	f("1:metric:foo", "10\n20\n30", []Row{
		{Metric: "foo", Value: 10},
		{Metric: "foo", Value: 20},
		{Metric: "foo", Value: 30},
	})
	f("1:label:env,2:metric:m,3:time:unix_s",
		"prod,42,1000\nstaging,99,2000",
		[]Row{
			{Metric: "m", Tags: []Tag{{Key: "env", Value: "prod"}}, Value: 42, Timestamp: 1000000},
			{Metric: "m", Tags: []Tag{{Key: "env", Value: "staging"}}, Value: 99, Timestamp: 2000000},
		})
}

func TestExportImportRoundTrip(t *testing.T) {
	format := "1:label:host,2:metric:cpu,3:time:unix_s"
	cds, err := ParseColumnDescriptors(format)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Simulated export output: header + data rows
	exported := strings.Join([]string{
		"host,value,timestamp",
		"server1,85.5,1704067200",
		"server2,92.3,1704067200",
		"server1,88.1,1704067260",
	}, "\n")

	var rs Rows
	rs.UnmarshalDetectHeader(exported, cds)
	expected := []Row{
		{Metric: "cpu", Tags: []Tag{{Key: "host", Value: "server1"}}, Value: 85.5, Timestamp: 1704067200000},
		{Metric: "cpu", Tags: []Tag{{Key: "host", Value: "server2"}}, Value: 92.3, Timestamp: 1704067200000},
		{Metric: "cpu", Tags: []Tag{{Key: "host", Value: "server1"}}, Value: 88.1, Timestamp: 1704067260000},
	}
	if !reflect.DeepEqual(rs.Rows, expected) {
		t.Fatalf("round-trip mismatch;\ngot\n%v\nwant\n%v", rs.Rows, expected)
	}

	// Without header detection the header line is an invalid row;
	// the 3 data rows are still parsed.
	rs.Reset()
	rs.Unmarshal(exported, cds)
	if len(rs.Rows) != 3 {
		t.Fatalf("expected 3 rows; got %d", len(rs.Rows))
	}
}
