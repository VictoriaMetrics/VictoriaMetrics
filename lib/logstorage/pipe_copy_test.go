package logstorage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prefixfilter"
)

func TestParsePipeCopySuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`copy foo as bar`)
	f(`copy foo as bar, a as b`)
	f(`copy * as foo.*`)
	f(`copy foo* as bar*`)
}

func TestParsePipeCopyFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`copy`)
	f(`copy x`)
	f(`copy x as`)
	f(`copy x y z`)
}

func TestPipeCopy(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, copy from existing field
	f("copy a as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
		},
	})

	// single row, copy from existing field to multiple fields
	f("copy a as b, a as c, _msg as d", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
			{"c", `test`},
			{"d", `{"foo":"bar"}`},
		},
	})

	// single row, copy from non-exsiting field
	f("copy x as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", ``},
		},
	})

	// copy to existing field
	f("copy _msg as a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `{"foo":"bar"}`},
		},
	})

	// copy to itself
	f("copy a as a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	})

	// swap copy
	f("copy a as b, _msg as a, b as _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `test`},
			{"a", `{"foo":"bar"}`},
			{"b", `test`},
		},
	})

	// copy to the same field multiple times
	f("copy a as b, _msg as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `{"foo":"bar"}`},
		},
	})

	// chain copy
	f("copy a as b, b as c", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
			{"c", `test`},
		},
	})

	// wildcard copy all the fields to themselves
	f("copy * as *", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	})

	// wildcard copy fields with some prefix to fields with another prefix
	f("copy a* as foo*", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
			{"foo", `test`},
			{"foobc", `aaa`},
		},
	})

	// wildcard copy fields with some prefix to fields without the prefix.
	f("copy a* as *", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
			{"", `test`},
			{"bc", `aaa`},
		},
	})

	// wildcard copy fields with some prefix to a single field
	f("copy a* as foo", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
			{"foo", `aaa`},
		},
	})

	// copy all the fields with prefix
	f("copy * as foo.*", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
			{"foo._msg", `{"foo":"bar"}`},
			{"foo.a", `test`},
			{"foo.abc", `aaa`},
		},
	})

	// Multiple rows
	f("copy a as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
		{
			{"a", `foobar`},
		},
		{
			{"b", `baz`},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"b", `test`},
		},
		{
			{"a", `foobar`},
			{"b", `foobar`},
		},
		{
			{"b", ``},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
			{"b", ""},
		},
	})
}

func TestPipeCopyUpdateNeededFields(t *testing.T) {
	f := func(s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f("copy s1 d1, s2 d2", "*", "", "*", "d1,d2")
	f("copy a a", "*", "", "*", "")
	f("copy foo* bar*", "*", "", "*", "bar*")
	f("copy foo bar*", "*", "", "*", "bar*")
	f("copy foo* bar", "*", "", "*", "bar")
	f("copy * bar*", "*", "", "*", "")
	f("copy b* bar*", "*", "", "*", "")
	f("copy bar* b*", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src and dst
	f("copy s1 d1, s2 d2", "*", "f1,f2", "*", "d1,d2,f1,f2")
	f("copy s1 d1, s2 d2", "*", "f*", "*", "d1,d2,f*")
	f("copy s1* d1*, s2 d2", "*", "f1,f2", "*", "d1*,d2,f1,f2")
	f("copy s1* d1*, s2 d2", "*", "f*", "*", "d1*,d2,f*")

	// all the needed fields, unneeded fields intersect with src
	f("copy s1 d1, s2 d2", "*", "s1,f1,f2", "*", "d1,d2,f1,f2")
	f("copy s1 d1, s2 d2", "*", "s*,f*", "*", "d1,d2,f*")
	f("copy s1* d1*, s2 d2", "*", "s1,f1,f2", "*", "d1*,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with dst
	f("copy s1 d1, s2 d2", "*", "d2,f1,f2", "*", "d1,d2,f1,f2")
	f("copy s1 d1, s2 d2", "*", "d*,f*", "*", "d*,f*")
	f("copy s1* d1*, s2 d2", "*", "d2,f1,f2", "*", "d1*,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with src and dst
	f("copy s1 d1, s2 d2", "*", "s1,d1,f1,f2", "*", "d1,d2,f1,f2,s1")
	f("copy s1 d1, s2 d2", "*", "s*,d*,f1,f2", "*", "d*,f1,f2,s*")
	f("copy s1* d1*, s2 d2", "*", "s1,d1,f1,f2", "*", "d1*,d2,f1,f2")
	f("copy s1 d1, s2 d2", "*", "s1,d2,f1,f2", "*", "d1,d2,f1,f2")
	f("copy s1 d1, s2 d2", "*", "s*,d*,f1,f2", "*", "d*,f1,f2,s*")
	f("copy s1* d1*, s2 d2", "*", "s1,d2,f1,f2", "*", "d1*,d2,f1,f2")

	// needed fields do not intersect with src and dst
	f("copy s1 d1, s2 d2", "f1,f2", "", "f1,f2", "")
	f("copy s1 d1, s2 d2", "f*", "", "f*", "")
	f("copy s1* d1*, s2 d2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("copy s1 d1, s2 d2", "s1,f1,f2", "", "s1,f1,f2", "")
	f("copy s1 d1, s2 d2", "s*,f*", "", "s*,f*", "")
	f("copy s1* d1*, s2 d2", "s1,f1,f2", "", "s1,f1,f2", "")

	// needed fields intersect with dst
	f("copy s1 d1, s2 d2", "d1,f1,f2", "", "f1,f2,s1", "")
	f("copy s1 d1, s2 d2", "d*,f*", "", "d*,f*,s1,s2", "d1,d2")
	f("copy s1* d1*, s2 d2", "d1,f1,f2", "", "f1,f2,s1*", "")
	f("copy s1* d1*, s2 d2", "d1*,f1,f2", "", "f1,f2,s1*", "")

	// needed fields intersect with src and dst
	f("copy s1 d1, s2 d2", "s1,d1,f1,f2", "", "s1,f1,f2", "")
	f("copy s1 d1, s2 d2", "s1,d2,f1,f2", "", "s1,s2,f1,f2", "")
	f("copy s1 d1, s2 d2", "s2,d1,f1,f2", "", "s1,s2,f1,f2", "")
	f("copy s1 d1, s2 d2", "s*,d*,f1,f2", "", "d*,f1,f2,s*", "d1,d2")
	f("copy s1* d1*, s2 d2", "s2,d1,f1,f2", "", "f1,f2,s1*,s2", "")
}

func expectPipeNeededFields(t *testing.T, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
	t.Helper()

	pf := newTestFieldsFilter(allowFilters, denyFilters)

	lex := newLexer(s, 0)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("cannot parse %s: %s", s, err)
	}
	p.updateNeededFields(pf)

	pfStr := pf.String()
	pfExpectedStr := fmt.Sprintf("allow=[%s], deny=[%s]", quoteStrings(allowFiltersExpected), quoteStrings(denyFiltersExpected))

	if pfStr != pfExpectedStr {
		t.Fatalf("unexpected field filters\ngot\n%s\nwant\n%s", pfStr, pfExpectedStr)
	}
}

func quoteStrings(s string) string {
	if s == "" {
		return ""
	}
	a := strings.Split(s, ",")
	tmp := make([]string, len(a))
	for i, v := range a {
		tmp[i] = strconv.Quote(v)
	}
	sort.Strings(tmp)
	return strings.Join(tmp, ",")
}

func newTestFieldsFilter(allowFilters, denyFilters string) *prefixfilter.Filter {
	var pf prefixfilter.Filter

	if allowFilters != "" {
		pf.AddAllowFilters(strings.Split(allowFilters, ","))
	}
	if denyFilters != "" {
		pf.AddDenyFilters(strings.Split(denyFilters, ","))
	}

	return &pf
}
