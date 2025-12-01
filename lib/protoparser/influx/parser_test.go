package influx

import (
	"reflect"
	"testing"
)

func TestNextUnquotedChar(t *testing.T) {
	f := func(s string, ch byte, noUnescape bool, nExpected int) {
		t.Helper()
		b := []byte(s)
		n := nextUnquotedChar(b, ch, noUnescape, true)
		if n != nExpected {
			t.Fatalf("unexpected n for nextUnquotedChar(%q, '%c', %v); got %d; want %d", s, ch, noUnescape, n, nExpected)
		}
	}

	f(``, ' ', false, -1)
	f(``, ' ', true, -1)
	f(`""`, ' ', false, -1)
	f(`""`, ' ', true, -1)
	f(`"foo bar\" " baz`, ' ', false, 12)
	f(`"foo bar\" " baz`, ' ', true, 10)
}

func TestNextUnescapedChar(t *testing.T) {
	f := func(s string, ch byte, noUnescape bool, nExpected int) {
		t.Helper()
		b := []byte(s)
		n := nextUnescapedChar(b, ch, noUnescape)
		if n != nExpected {
			t.Fatalf("unexpected n for nextUnescapedChar(%q, '%c', %v); got %d; want %d", s, ch, noUnescape, n, nExpected)
		}
	}

	f("", ' ', true, -1)
	f("", ' ', false, -1)
	f(" ", ' ', true, 0)
	f(" ", ' ', false, 0)
	f("x y", ' ', true, 1)
	f("x y", ' ', false, 1)
	f(`x\  y`, ' ', true, 2)
	f(`x\  y`, ' ', false, 3)
	f(`\\,`, ',', true, 2)
	f(`\\,`, ',', false, 2)
	f(`\\\=`, '=', true, 3)
	f(`\\\=`, '=', false, -1)
	f(`\\\=aa`, '=', true, 3)
	f(`\\\=aa`, '=', false, -1)
	f(`\\\=a=a`, '=', true, 3)
	f(`\\\=a=a`, '=', false, 5)
	f(`a\`, ' ', true, -1)
	f(`a\`, ' ', false, -1)
}

func TestUnescapeTagValue(t *testing.T) {
	f := func(s, sExpected string) {
		t.Helper()
		b := []byte(s)
		ss := unescapeTagValue(b, false)
		if string(ss) != sExpected {
			t.Fatalf("unexpected value for %q; got %q; want %q", s, ss, sExpected)
		}
	}

	f("", "")
	f("x", "x")
	f("foobar", "foobar")
	f("привет", "привет")
	f(`\a\b\cd`, `\a\b\cd`)
	f(`\`, `\`)
	f(`foo\`, `foo\`)
	f(`\,foo\\\=\ bar`, `,foo\= bar`)
}

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		b := []byte(s)
		err := rows.Unmarshal(b)
		if err == nil {
			t.Fatal("unexpected nil error")
		}
		if len(rows.Rows) != 0 {
			t.Fatalf("expecting zero rows; got %d rows", len(rows.Rows))
		}

		// Try again
		b = []byte(s)
		_ = rows.Unmarshal(b)
		if len(rows.Rows) != 0 {
			t.Fatalf("expecting zero rows; got %d rows", len(rows.Rows))
		}
	}

	// No fields
	f("foo")
	f("foo,bar=baz 1234")

	// Missing tag value
	f("foo,bar")
	f("foo,bar baz")
	f("foo,bar=123, 123")

	// Missing field value
	f("foo bar")
	f("foo bar=")
	f("foo bar=,baz=23 123")
	f("foo bar=1, 123")
	f(`foo bar=" 123`)
	f(`foo bar="123`)
	f(`foo bar=",123`)
	f(`foo bar=a"", 123`)

	// Missing field name
	f("foo =123")
	f("foo =123\nbar")

	// Invalid timestamp
	f("foo bar=123 baz")

	// Invalid field value
	f("foo bar=1abci")
	f("foo bar=-2abci")
	f("foo bar=3abcu")

	// HTTP request line
	f("GET /foo HTTP/1.1")
	f("GET /foo?bar=baz HTTP/1.0")
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		rows := Rows{IgnoreErrs: true}
		b := []byte(s)
		err := rows.Unmarshal(b)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		b = []byte(s)
		_ = rows.Unmarshal(b)
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
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})

	// Comment
	f("\n# foobar\n", &Rows{})
	f("#foobar baz", &Rows{})
	f("#foobar baz\n#sss", &Rows{})

	// Missing measurement
	f(" baz=123", &Rows{
		Rows: []Row{{
			Measurement: []byte(""),
			Fields: []Field{{
				Key:   []byte("baz"),
				Value: 123,
			}},
		}},
	})
	f(",foo=bar baz=123", &Rows{
		Rows: []Row{{
			Measurement: []byte(""),
			Tags: []Tag{{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			}},
			Fields: []Field{{
				Key:   []byte("baz"),
				Value: 123,
			}},
		}},
	})

	// Minimal line without tags and timestamp
	f("foo bar=123", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})
	// Excess whitespace after final field. Issue #10049
	f("foo bar=123   ", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})
	f("# comment\nfoo bar=123\r\n#comment2 sdsf dsf", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})
	f("foo bar=123\n", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})

	// Line without tags and with a timestamp.
	f("foo bar=123.45 -345", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123.45,
			}},
			Timestamp: -345,
		}},
	})

	// Line with a single tag
	f("foo,tag1=xyz bar=123", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Tags: []Tag{{
				Key:   []byte("tag1"),
				Value: []byte("xyz"),
			}},
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})

	// Line with multiple tags
	f("foo,tag1=xyz,tag2=43as bar=123", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Tags: []Tag{
				{
					Key:   []byte("tag1"),
					Value: []byte("xyz"),
				},
				{
					Key:   []byte("tag2"),
					Value: []byte("43as"),
				},
			},
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})

	// Line with empty tag values
	f("foo,tag1=xyz,tagN=,tag2=43as,=xxx bar=123", &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Tags: []Tag{
				{
					Key:   []byte("tag1"),
					Value: []byte("xyz"),
				},
				{
					Key:   []byte("tag2"),
					Value: []byte("43as"),
				},
			},
			Fields: []Field{{
				Key:   []byte("bar"),
				Value: 123,
			}},
		}},
	})

	// Line with multiple tags, multiple fields and timestamp
	f(`system,host=ip-172-16-10-144 uptime_format="3 days, 21:01",quoted_float="-1.23",quoted_int="123" 1557761040000000000`, &Rows{
		Rows: []Row{{
			Measurement: []byte("system"),
			Tags: []Tag{{
				Key:   []byte("host"),
				Value: []byte("ip-172-16-10-144"),
			}},
			Fields: []Field{
				{
					Key:   []byte("uptime_format"),
					Value: 0,
				},
				{
					Key:   []byte("quoted_float"),
					Value: -1.23,
				},
				{
					Key:   []byte("quoted_int"),
					Value: 123,
				},
			},
			Timestamp: 1557761040000000000,
		}},
	})
	f(`foo,tag1=xyz,tag2=43as bar=-123e4,x=True,y=-45i,z=f,aa="f,= \"a",bb=23u 48934`, &Rows{
		Rows: []Row{{
			Measurement: []byte("foo"),
			Tags: []Tag{
				{
					Key:   []byte("tag1"),
					Value: []byte("xyz"),
				},
				{
					Key:   []byte("tag2"),
					Value: []byte("43as"),
				},
			},
			Fields: []Field{
				{
					Key:   []byte("bar"),
					Value: -123e4,
				},
				{
					Key:   []byte("x"),
					Value: 1,
				},
				{
					Key:   []byte("y"),
					Value: -45,
				},
				{
					Key:   []byte("z"),
					Value: 0,
				},
				{
					Key:   []byte("aa"),
					Value: 0,
				},
				{
					Key:   []byte("bb"),
					Value: 23,
				},
			},
			Timestamp: 48934,
		}},
	})

	// Escape chars
	f(`fo\,bar\=b\ az,x\=\ b=\\a\,\=\q\  \\\a\ b\=\,=4.34`, &Rows{
		Rows: []Row{{
			Measurement: []byte(`fo,bar=b az`),
			Tags: []Tag{{
				Key:   []byte(`x= b`),
				Value: []byte(`\a,=\q `),
			}},
			Fields: []Field{{
				Key:   []byte(`\\a b=,`),
				Value: 4.34,
			}},
		}},
	})
	// Test case from https://community.librenms.org/t/integration-with-victoriametrics/9689
	f("ports,foo=a,bar=et\\ +\\ V,baz=ype INDISCARDS=245333676,OUTDISCARDS=1798680", &Rows{
		Rows: []Row{{
			Measurement: []byte("ports"),
			Tags: []Tag{
				{
					Key:   []byte("foo"),
					Value: []byte("a"),
				},
				{
					Key:   []byte("bar"),
					Value: []byte("et + V"),
				},
				{
					Key:   []byte("baz"),
					Value: []byte("ype"),
				},
			},
			Fields: []Field{
				{
					Key:   []byte("INDISCARDS"),
					Value: 245333676,
				},
				{
					Key:   []byte("OUTDISCARDS"),
					Value: 1798680,
				},
			},
		}},
	})

	// Multiple lines
	f("foo,tag=xyz field=1.23 48934\n"+
		"bar x=-1i\n\n", &Rows{
		Rows: []Row{
			{
				Measurement: []byte("foo"),
				Tags: []Tag{{
					Key:   []byte("tag"),
					Value: []byte("xyz"),
				}},
				Fields: []Field{{
					Key:   []byte("field"),
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: []byte("bar"),
				Fields: []Field{{
					Key:   []byte("x"),
					Value: -1,
				}},
			},
		},
	})

	// Multiple lines with invalid line in the middle.
	f("foo,tag=xyz field=1.23 48934\n"+
		"invalid line\n"+
		"bar x=-1i\n\n", &Rows{
		Rows: []Row{
			{
				Measurement: []byte("foo"),
				Tags: []Tag{{
					Key:   []byte("tag"),
					Value: []byte("xyz"),
				}},
				Fields: []Field{{
					Key:   []byte("field"),
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: []byte("bar"),
				Fields: []Field{{
					Key:   []byte("x"),
					Value: -1,
				}},
			},
		},
	})

	// No newline after the second line.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/82
	f("foo,tag=xyz field=1.23 48934\n"+
		"bar x=-1i", &Rows{
		Rows: []Row{
			{
				Measurement: []byte("foo"),
				Tags: []Tag{{
					Key:   []byte("tag"),
					Value: []byte("xyz"),
				}},
				Fields: []Field{{
					Key:   []byte("field"),
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: []byte("bar"),
				Fields: []Field{{
					Key:   []byte("x"),
					Value: -1,
				}},
			},
		},
	})

	// Superfluous whitespace between tags, fields and timestamps.
	f(`cpu_utilization,host=mnsbook-pro.local value=119.8 1607222595591`, &Rows{
		Rows: []Row{{
			Measurement: []byte("cpu_utilization"),
			Tags: []Tag{{
				Key:   []byte("host"),
				Value: []byte("mnsbook-pro.local"),
			}},
			Fields: []Field{{
				Key:   []byte("value"),
				Value: 119.8,
			}},
			Timestamp: 1607222595591,
		}},
	})
	f(`cpu_utilization,host=mnsbook-pro.local   value=119.8   1607222595591`, &Rows{
		Rows: []Row{{
			Measurement: []byte("cpu_utilization"),
			Tags: []Tag{{
				Key:   []byte("host"),
				Value: []byte("mnsbook-pro.local"),
			}},
			Fields: []Field{{
				Key:   []byte("value"),
				Value: 119.8,
			}},
			Timestamp: 1607222595591,
		}},
	})

	f("x,y=z,g=p:\\ \\ 5432\\,\\ gp\\ mon\\ [lol]\\ con10\\ cmd5\\ SELECT f=1", &Rows{
		Rows: []Row{{
			Measurement: []byte("x"),
			Tags: []Tag{
				{
					Key:   []byte("y"),
					Value: []byte("z"),
				},
				{
					Key:   []byte("g"),
					Value: []byte("p:  5432, gp mon [lol] con10 cmd5 SELECT"),
				},
			},
			Fields: []Field{{
				Key:   []byte("f"),
				Value: 1,
			}},
		}},
	})
}
