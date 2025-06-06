package logstorage

import (
	"testing"
)

func TestParsePipeRenameSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`rename foo as bar`)
	f(`rename foo as bar, a as b`)
	f(`rename * as foo*`)
	f(`rename foo* as bar*`)
}

func TestParsePipeRenameFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`rename`)
	f(`rename x`)
	f(`rename x as`)
	f(`rename x y z`)
}

func TestPipeRename(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, rename from existing field
	f("rename a as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"b", `test`},
		},
	})

	// single row, rename from existing field to multiple fields
	f("rename a as b, a as c, _msg as d", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"b", `test`},
			{"c", ``},
			{"d", `{"foo":"bar"}`},
		},
	})

	// single row, rename from non-exsiting field
	f("rename x as b", [][]Field{
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

	// rename to existing field
	f("rename _msg as a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `{"foo":"bar"}`},
		},
	})

	// rename to itself
	f("rename a as a", [][]Field{
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

	// swap rename
	f("rename a as b, _msg as a, b as _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `test`},
			{"a", `{"foo":"bar"}`},
		},
	})

	// rename to the same field multiple times
	f("rename a as b, _msg as b", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"b", `{"foo":"bar"}`},
		},
	})

	// chain rename (shouldn't work - otherwise swap rename will break)
	f("rename a as b, b as c", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"c", `test`},
		},
	})

	// wildcard rename the same set of fields
	f("rename * as *", [][]Field{
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

	// wildcard rename fields with some prefix to fields without the prefix
	f("rename a* as *", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"", `test`},
			{"bc", `aaa`},
		},
	})

	// wildcard rename fields with some prefix to a signle field
	f("rename a* as foo", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"foo", `aaa`},
		},
	})

	// wildcard rename fields with some prefix to another prefix
	f("rename a* as foo*", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"foo", `test`},
			{"foobc", `aaa`},
		},
	})

	// rename all the fields with prefix
	f("rename * as foo.*", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
			{"abc", `aaa`},
		},
	}, [][]Field{
		{
			{"foo._msg", `{"foo":"bar"}`},
			{"foo.a", `test`},
			{"foo.abc", `aaa`},
		},
	})

	// Multiple rows
	f("rename a as b", [][]Field{
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
			{"b", `test`},
		},
		{
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

func TestPipeRenameUpdateNeededFields(t *testing.T) {
	f := func(s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f("rename s1 d1, s2 d2", "*", "", "*", "d1,d2")
	f("rename a a", "*", "", "*", "")
	f("rename a* b*", "*", "", "*", "b*")
	f("rename a* a*", "*", "", "*", "")
	f("rename abc* a*", "*", "", "*", "")
	f("rename a* abc*", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src and dst
	f("rename s1 d1, s2 d2", "*", "f1,f2", "*", "d1,d2,f1,f2")
	f("rename s1 d1, s2 d2", "*", "f*", "*", "d1,d2,f*")
	f("rename s1* d1*, s2 d2", "*", "f1,f2", "*", "d1*,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with src
	// mv s1 d1, s2 d2 | rm s1, f1, f2   (d1, d2, f1, f2)
	f("rename s1 d1, s2 d2", "*", "s1,f1,f2", "*", "d1,d2,f1,f2")
	f("rename s1 d1, s2 d2", "*", "s*,f*", "*", "d1,d2,f*")
	f("rename s1* d1*, s2 d2", "*", "s1,f1,f2", "*", "d1*,d2,f1,f2")

	// all the needed fields, unneeded fields intersect with dst
	f("rename s1 d1, s2 d2", "*", "d2,f1,f2", "*", "d1,d2,f1,f2,s2")
	f("rename s1 d1, s2 d2", "*", "d*,f*", "*", "d*,f*,s1,s2")
	f("rename s1* d1*, s2 d2", "*", "d2,f1,f2", "*", "d1*,d2,f1,f2,s2")

	// all the needed fields, unneeded fields intersect with src and dst
	f("rename s1 d1, s2 d2", "*", "s1,d1,f1,f2", "*", "d1,d2,f1,f2,s1")
	f("rename s1 d1, s2 d2", "*", "s*,d*,f*", "*", "d*,f*,s*")
	f("rename s1* d1*, s2 d2", "*", "s1,d1,f1,f2", "*", "d1*,d2,f1,f2")
	f("rename s1 d1, s2 d2", "*", "s1,d2,f1,f2", "*", "d1,d2,f1,f2,s2")
	f("rename s1 d1, s2 d2", "*", "s*,d*,f*", "*", "d*,f*,s*")
	f("rename s1* d1*, s2 d2", "*", "s1,d2,f1,f2", "*", "d1*,d2,f1,f2,s2")

	// needed fields do not intersect with src and dst
	f("rename s1 d1, s2 d2", "f1,f2", "", "f1,f2", "")
	f("rename s1 d1, s2 d2", "f*", "", "f*", "")
	f("rename s1* d1*, s2 d2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("rename s1 d1, s2 d2", "s1,f1,f2", "", "f1,f2", "")
	f("rename s1 d1, s2 d2", "s*,f*", "", "f*,s*", "s1,s2")
	f("rename s1* d1*, s2 d2", "s1,f1,f2", "", "f1,f2", "")

	// needed fields intersect with dst
	f("rename s1 d1, s2 d2", "d1,f1,f2", "", "f1,f2,s1", "")
	f("rename s1 d1, s2 d2", "d*,f*", "", "d*,f*,s1,s2", "d1,d2")
	f("rename s1* d1*, s2 d2", "d1,f1,f2", "", "f1,f2,s1*", "")

	// needed fields intersect with src and dst
	f("rename s1 d1, s2 d2", "s1,d1,f1,f2", "", "s1,f1,f2", "")
	f("rename s1 d1, s2 d2", "s1,d2,f1,f2", "", "s2,f1,f2", "")
	f("rename s1 d1, s2 d2", "s2,d1,f1,f2", "", "s1,f1,f2", "")
	f("rename s1 d1, s2 d2", "s*,d*,f*", "", "d*,f*,s*", "d1,d2")
	f("rename s1* d1*, s2 d2", "s2,d1,f1,f2", "", "s1*,f1,f2", "")
}
