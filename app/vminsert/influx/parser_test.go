package influx

import (
	"reflect"
	"testing"
)

func TestNextUnquotedChar(t *testing.T) {
	f := func(s string, ch byte, noUnescape bool, nExpected int) {
		t.Helper()
		n := nextUnquotedChar(s, ch, noUnescape, true)
		if n != nExpected {
			t.Fatalf("unexpected n for nextUnqotedChar(%q, '%c', %v); got %d; want %d", s, ch, noUnescape, n, nExpected)
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
		n := nextUnescapedChar(s, ch, noUnescape)
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
		ss := unescapeTagValue(s, false)
		if ss != sExpected {
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
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("expecting zero rows; got %d rows", len(rows.Rows))
		}

		// Try again
		rows.Unmarshal(s)
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
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})

	// Comment
	f("\n# foobar\n", &Rows{})
	f("#foobar baz", &Rows{})
	f("#foobar baz\n#sss", &Rows{})

	// Missing measurement
	f(" baz=123", &Rows{
		Rows: []Row{{
			Measurement: "",
			Fields: []Field{{
				Key:   "baz",
				Value: 123,
			}},
		}},
	})
	f(",foo=bar baz=123", &Rows{
		Rows: []Row{{
			Measurement: "",
			Tags: []Tag{{
				Key:   "foo",
				Value: "bar",
			}},
			Fields: []Field{{
				Key:   "baz",
				Value: 123,
			}},
		}},
	})

	// Minimal line without tags and timestamp
	f("foo bar=123", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})
	f("# comment\nfoo bar=123\r\n#comment2 sdsf dsf", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})
	f("foo bar=123\n", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})

	// Line without tags and with a timestamp.
	f("foo bar=123.45 -345", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Fields: []Field{{
				Key:   "bar",
				Value: 123.45,
			}},
			Timestamp: -345,
		}},
	})

	// Line with a single tag
	f("foo,tag1=xyz bar=123", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Tags: []Tag{{
				Key:   "tag1",
				Value: "xyz",
			}},
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})

	// Line with multiple tags
	f("foo,tag1=xyz,tag2=43as bar=123", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Tags: []Tag{
				{
					Key:   "tag1",
					Value: "xyz",
				},
				{
					Key:   "tag2",
					Value: "43as",
				},
			},
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})

	// Line with empty tag values
	f("foo,tag1=xyz,tagN=,tag2=43as,=xxx bar=123", &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Tags: []Tag{
				{
					Key:   "tag1",
					Value: "xyz",
				},
				{
					Key:   "tag2",
					Value: "43as",
				},
			},
			Fields: []Field{{
				Key:   "bar",
				Value: 123,
			}},
		}},
	})

	// Line with multiple tags, multiple fields and timestamp
	f(`system,host=ip-172-16-10-144 uptime_format="3 days, 21:01",quoted_float="-1.23",quoted_int="123" 1557761040000000000`, &Rows{
		Rows: []Row{{
			Measurement: "system",
			Tags: []Tag{{
				Key:   "host",
				Value: "ip-172-16-10-144",
			}},
			Fields: []Field{
				{
					Key:   "uptime_format",
					Value: 0,
				},
				{
					Key:   "quoted_float",
					Value: -1.23,
				},
				{
					Key:   "quoted_int",
					Value: 123,
				},
			},
			Timestamp: 1557761040000000000,
		}},
	})
	f(`foo,tag1=xyz,tag2=43as bar=-123e4,x=True,y=-45i,z=f,aa="f,= \"a",bb=23u 48934`, &Rows{
		Rows: []Row{{
			Measurement: "foo",
			Tags: []Tag{
				{
					Key:   "tag1",
					Value: "xyz",
				},
				{
					Key:   "tag2",
					Value: "43as",
				},
			},
			Fields: []Field{
				{
					Key:   "bar",
					Value: -123e4,
				},
				{
					Key:   "x",
					Value: 1,
				},
				{
					Key:   "y",
					Value: -45,
				},
				{
					Key:   "z",
					Value: 0,
				},
				{
					Key:   "aa",
					Value: 0,
				},
				{
					Key:   "bb",
					Value: 23,
				},
			},
			Timestamp: 48934,
		}},
	})

	// Escape chars
	f(`fo\,bar\=baz,x\=\b=\\a\,\=\q\  \\\a\=\,=4.34`, &Rows{
		Rows: []Row{{
			Measurement: `fo,bar=baz`,
			Tags: []Tag{{
				Key:   `x=\b`,
				Value: `\a,=\q `,
			}},
			Fields: []Field{{
				Key:   `\\a=,`,
				Value: 4.34,
			}},
		}},
	})
	// Test case from https://community.librenms.org/t/integration-with-victoriametrics/9689
	f("ports,foo=a,bar=et\\ +\\ V,baz=ype INDISCARDS=245333676,OUTDISCARDS=1798680", &Rows{
		Rows: []Row{{
			Measurement: "ports",
			Tags: []Tag{
				{
					Key:   "foo",
					Value: "a",
				},
				{
					Key:   "bar",
					Value: "et + V",
				},
				{
					Key:   "baz",
					Value: "ype",
				},
			},
			Fields: []Field{
				{
					Key:   "INDISCARDS",
					Value: 245333676,
				},
				{
					Key:   "OUTDISCARDS",
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
				Measurement: "foo",
				Tags: []Tag{{
					Key:   "tag",
					Value: "xyz",
				}},
				Fields: []Field{{
					Key:   "field",
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: "bar",
				Fields: []Field{{
					Key:   "x",
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
				Measurement: "foo",
				Tags: []Tag{{
					Key:   "tag",
					Value: "xyz",
				}},
				Fields: []Field{{
					Key:   "field",
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: "bar",
				Fields: []Field{{
					Key:   "x",
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
				Measurement: "foo",
				Tags: []Tag{{
					Key:   "tag",
					Value: "xyz",
				}},
				Fields: []Field{{
					Key:   "field",
					Value: 1.23,
				}},
				Timestamp: 48934,
			},
			{
				Measurement: "bar",
				Fields: []Field{{
					Key:   "x",
					Value: -1,
				}},
			},
		},
	})

	f("x,y=z,g=p:\\ \\ 5432\\,\\ gp\\ mon\\ [lol]\\ con10\\ cmd5\\ SELECT f=1", &Rows{
		Rows: []Row{{
			Measurement: "x",
			Tags: []Tag{
				{
					Key:   "y",
					Value: "z",
				},
				{
					Key:   "g",
					Value: "p:  5432, gp mon [lol] con10 cmd5 SELECT",
				},
			},
			Fields: []Field{{
				Key:   "f",
				Value: 1,
			}},
		}},
	})
}
