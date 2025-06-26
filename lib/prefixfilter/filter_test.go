package prefixfilter

import (
	"reflect"
	"testing"
)

func TestIsWildcardFilter(t *testing.T) {
	f := func(filter string, resultExpected bool) {
		t.Helper()

		result := IsWildcardFilter(filter)
		if result != resultExpected {
			t.Fatalf("unexpected result for IsWildcardFilter(%q); got %v; want %v", filter, result, resultExpected)
		}
	}

	f("", false)
	f("foo", false)
	f("*", true)
	f("foo*", true)
	f("*f", false)
	f("*f*", true)
}

func TestMatchFilter(t *testing.T) {
	f := func(filter, s string, resultExpected bool) {
		t.Helper()

		result := MatchFilter(filter, s)
		if result != resultExpected {
			t.Fatalf("unexpected result for MatchFilter(%q, %q); got %v; want %v", filter, s, result, resultExpected)
		}
	}

	f("", "", true)
	f("", "foo", false)
	f("foo", "", false)
	f("foo", "foo", true)
	f("foo", "foobar", false)
	f("foo", "bar", false)

	f("*", "", true)
	f("*", "foo", true)
	f("a*", "", false)
	f("a*", "a", true)
	f("a*", "abc", true)
	f("a*", "foo", false)
}

func TestMatchFilters(t *testing.T) {
	f := func(filters []string, s string, resultExpected bool) {
		t.Helper()

		result := MatchFilters(filters, s)
		if result != resultExpected {
			t.Fatalf("unexpected result for MatchFilters(%q, %q); got %v; want %v", filters, s, result, resultExpected)
		}
	}

	f(nil, "", false)
	f(nil, "foo", false)
	f([]string{""}, "", true)
	f([]string{""}, "foo", false)
	f([]string{"foo"}, "", false)
	f([]string{"foo", ""}, "", true)
	f([]string{"foo", "ba*"}, "", false)
	f([]string{"foo", "ba*"}, "foo", true)
	f([]string{"foo", "ba*"}, "foobar", false)
	f([]string{"foo", "ba*"}, "ba", true)
	f([]string{"foo", "ba*"}, "bar", true)
}

func TestMatchAll(t *testing.T) {
	f := func(filters []string, resultExpected bool) {
		t.Helper()

		result := MatchAll(filters)
		if result != resultExpected {
			t.Fatalf("unexpected result for MatchAll(%q); got %v; want %v", filters, result, resultExpected)
		}
	}

	f(nil, false)
	f([]string{"foo"}, false)
	f([]string{"foo", "bar*"}, false)
	f([]string{"foo", "*", "abc"}, true)
	f([]string{"*"}, true)
}

func TestAppendReplace(t *testing.T) {
	f := func(srcFilter, dstFilter, s, resultExpected string) {
		t.Helper()

		result := AppendReplace(nil, srcFilter, dstFilter, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for AppendReplace(%q, %q, %q); got %q; want %q", srcFilter, dstFilter, s, result, resultExpected)
		}
	}

	// Full string
	f("", "", "", "")
	f("", "", "foo", "foo")
	f("foo", "bar", "baz", "baz")
	f("foo", "bar", "foo", "bar")

	// Prefix only at srcFilter
	f("foo.*", "bar", "foo", "foo")
	f("foo.*", "bar", "foo.", "bar")
	f("foo.*", "bar", "foo.xyz", "bar")

	// Prefix only at dstFilter
	f("foo", "bar.*", "a", "a")
	f("foo", "bar.*", "foo", "bar.*")
	f("foo", "bar.*", "foo.", "foo.")

	// Prefix at both srcFilter and dstFilter
	f("foo.*", "bar.baz.*", "foo", "foo")
	f("foo.*", "bar.baz.*", "foo.", "bar.baz.")
	f("foo.*", "bar.baz.*", "foo.x", "bar.baz.x")
	f("foo.*", "bar.baz.*", "foo.xyz", "bar.baz.xyz")
}

func TestFilter_MatchNothing(t *testing.T) {
	var f Filter

	if !f.MatchNothing() {
		t.Fatalf("MatchNothing must return true for empty filter")
	}

	// Allow some
	f.AddAllowFilters([]string{"foo", "bar*"})
	if f.MatchNothing() {
		t.Fatalf("MatchNothing must return false for non-empty filter")
	}

	// Deny some
	f.AddDenyFilters([]string{"abc", "def*"})
	if f.MatchNothing() {
		t.Fatalf("MatchNothing must return false for non-empty filter")
	}

	// Deny all
	f.AddDenyFilter("*")
	if !f.MatchNothing() {
		t.Fatalf("MatchNothing must return true for empty filter")
	}

	// Allow some and then reset
	f.AddAllowFilter("foo*")
	f.AddAllowFilter("bar")
	f.Reset()
	if !f.MatchNothing() {
		t.Fatalf("MatchNothing must return true for empty filter")
	}
}

func TestFilter_MatchAll(t *testing.T) {
	var f Filter

	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for empty filter")
	}

	f.AddAllowFilter("foo")
	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for filter without *")
	}

	f.AddAllowFilter("bar*")
	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for filter without *")
	}

	f.AddAllowFilter("*")
	if !f.MatchAll() {
		t.Fatalf("MatchAll() must return true for * filter")
	}

	f.AddDenyFilter("foo")
	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for filter with non-empty deny filters")
	}

	f.AddDenyFilter("bar*")
	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for filter with non-empty deny filters")
	}

	f.AddAllowFilter("*")
	if !f.MatchAll() {
		t.Fatalf("MatchAll() must return true for * filter")
	}

	f.Reset()
	if f.MatchAll() {
		t.Fatalf("MatchAll() must return false for empty filter")
	}
}

func TestFilter_MatchString_NilFilter(t *testing.T) {
	f := func(s string) {
		t.Helper()

		var f *Filter
		if f.MatchString(s) {
			t.Fatalf("unexpected MatchString(%q) for nil Filter; got true; want false", s)
		}
	}

	f("")
	f("foo")
}

func TestFilter_Clone(t *testing.T) {
	f := func(allow, deny []string) {
		t.Helper()

		var f Filter
		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)
		fCopy := f.Clone()

		fStr := f.String()
		fCopyStr := fCopy.String()

		if fStr != fCopyStr {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", fStr, fCopyStr)
		}
	}

	f(nil, nil)
	f([]string{"foo", "bar*"}, nil)
	f([]string{"foo", "bar*"}, []string{"baz", "x*"})
}

func TestFilter_GetAllowStrings(t *testing.T) {
	f := func(allow, deny, resultExpected []string, okExpected bool) {
		t.Helper()

		var f Filter

		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)

		result, ok := f.GetAllowStrings()

		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
		if ok != okExpected {
			t.Fatalf("unexpected ok; got %v; want %v", ok, okExpected)
		}
	}

	f(nil, nil, nil, true)
	f([]string{"*"}, nil, nil, false)
	f([]string{"foo", "bar", "baz*"}, nil, nil, false)
	f([]string{"foo", "bar"}, nil, []string{"foo", "bar"}, true)
	f([]string{"foo", "bar"}, []string{"foobar*"}, []string{"foo", "bar"}, true)
	f([]string{"foo", "bar"}, []string{"fo*"}, []string{"bar"}, true)
}

func TestFilter_GetAllowFilters(t *testing.T) {
	f := func(allow, deny, resultExpected []string) {
		t.Helper()

		var f Filter

		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)

		result := f.GetAllowFilters()
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
	}

	f(nil, nil, []string{})
	f([]string{"*"}, nil, []string{"*"})
	f([]string{"foo", "bar*"}, nil, []string{"bar*", "foo"})
	f([]string{"foo", "*"}, nil, []string{"*"})
	f([]string{"foo", "bar*"}, []string{"barz", "f*"}, []string{"bar*"})
	f([]string{"*"}, []string{"*"}, []string{})
	f([]string{"*"}, []string{"foo*"}, []string{"*"})
}

func TestFilter_GetDenyFilters(t *testing.T) {
	f := func(allow, deny, resultExpected []string) {
		t.Helper()

		var f Filter

		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)

		result := f.GetDenyFilters()
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", result, resultExpected)
		}
	}

	f(nil, nil, []string{})
	f([]string{"*"}, nil, []string{})
	f(nil, []string{"foo", "bar*"}, []string{})
	f([]string{"*"}, []string{"foo", "bar*"}, []string{"bar*", "foo"})
	f(nil, []string{"foo", "*"}, []string{})
	f([]string{"*"}, []string{"foo", "*"}, []string{})
	f([]string{"foo"}, []string{"f*", "barz", "f*"}, []string{})
	f([]string{"foo", "bar*"}, []string{"f*", "barz", "f*"}, []string{"barz", "f*"})

	// Zero intersection between allow and deny filters
	f([]string{"foo"}, []string{"bar"}, []string{})
	f([]string{"foo*"}, []string{"bar"}, []string{})
	f([]string{"foo"}, []string{"bar*"}, []string{})
	f([]string{"foo*"}, []string{"bar*"}, []string{})
}

func TestFilter_MatchStringOrWildcard(t *testing.T) {
	f := func(allow, deny []string, s string, resultExpected bool) {
		t.Helper()

		var f Filter

		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)

		result := f.MatchStringOrWildcard(s)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	// Empty allow
	f(nil, nil, "", false)
	f(nil, nil, "foo", false)
	f(nil, nil, "*", false)
	f(nil, nil, "foo*", false)

	// Allow all
	f([]string{"*"}, nil, "", true)
	f([]string{"*"}, nil, "foo", true)
	f([]string{"*", "a", "b*"}, nil, "", true)
	f([]string{"*", "a", "b*"}, nil, "foo", true)
	f([]string{"*", "a", "b*"}, nil, "*", true)
	f([]string{"*", "a", "b*"}, nil, "foo*", true)

	// Allow all, deny some
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "foo", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bar", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "baz", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bam", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bamp", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "abc", true)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "*", true)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "f*", true)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "ba*", false)

	// Deny all
	f([]string{"*"}, []string{"*"}, "", false)
	f([]string{"*"}, []string{"*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "*", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "b*", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "ba*", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "bar*", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "f*", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "foo*", false)

	// Allow some
	f([]string{"foo", "ba*"}, nil, "", false)
	f([]string{"foo", "ba*"}, nil, "foo", true)
	f([]string{"foo", "ba*"}, nil, "foobar", false)
	f([]string{"foo", "ba*"}, nil, "ba", true)
	f([]string{"foo", "ba*"}, nil, "bar", true)
	f([]string{"foo", "ba*"}, nil, "abc", false)
	f([]string{"foo", "ba*"}, nil, "*", true)
	f([]string{"foo", "ba*"}, nil, "f*", true)
	f([]string{"foo", "ba*"}, nil, "foo*", true)
	f([]string{"foo", "ba*"}, nil, "z*", false)
	f([]string{"foo", "ba*"}, nil, "b*", true)
	f([]string{"foo", "ba*"}, nil, "ba*", true)
	f([]string{"foo", "ba*"}, nil, "bar*", true)

	// Mix allow / deny
	f([]string{"foo", "ba*"}, []string{"bar"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"bar"}, "foo", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"bar"}, "baz", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "barz", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "*", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "f*", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "foo*", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "b*", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "ba*", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "bar*", true)
	f([]string{"foo", "ba*"}, []string{"bar*"}, "ba*", true)
	f([]string{"foo", "ba*"}, []string{"bar*"}, "bar*", false)
	f([]string{"foo", "ba*"}, []string{"bar*"}, "barz*", false)

	// Deny overrides everything
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "ba", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "*", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "f*", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "foo*", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "b*", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "ba*", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "bar*", false)

	// Deny overrides some
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "foo", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "baz", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "barz", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "bam", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "*", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "b*", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "ba*", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "bar*", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "barz*", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "baz*", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "fo*", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "foo*", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "foobar*", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "zoo*", false)

	// Deny equals allow
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "foo", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "bar", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "abc", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "foo", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "bar", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "abc", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "*", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "foo*", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "ba*", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "bar*", false)
}

func TestFilter_DropBroaderDenyFilters(t *testing.T) {
	f := func(deny, allow, denyExpected, allowExpected []string) {
		t.Helper()

		var f Filter
		f.AddAllowFilter("*")
		f.AddDenyFilters(deny)
		f.AddAllowFilters(allow)

		denyResult := f.GetDenyFilters()
		allowResult := f.GetAllowFilters()

		if !reflect.DeepEqual(denyResult, denyExpected) {
			t.Fatalf("unexpected deny filters\ngot\n%q\nwant\n%q", denyResult, denyExpected)
		}
		if !reflect.DeepEqual(allowResult, allowExpected) {
			t.Fatalf("unexpected allow filters\ngot\n%q\nwant\n%q", allowResult, allowExpected)
		}
	}

	f([]string{"*"}, []string{"foo"}, []string{}, []string{"foo"})
	f([]string{"*"}, []string{"foo*"}, []string{}, []string{"foo*"})
	f([]string{"*"}, []string{"ab", "foo*"}, []string{}, []string{"ab", "foo*"})
	f([]string{"a*", "b"}, []string{"foo"}, []string{"a*", "b"}, []string{"*"})
	f([]string{"a*", "b"}, []string{"foo*", "abc"}, []string{"b"}, []string{"*"})
	f([]string{"a*", "b"}, []string{"*"}, []string{}, []string{"*"})
	f([]string{"a*", "b"}, []string{"b*"}, []string{"a*"}, []string{"*"})
	f([]string{"a*", "b"}, []string{"b*", "a"}, []string{}, []string{"*"})
	f([]string{"a*", "b"}, []string{"bc*", "ab"}, []string{"b"}, []string{"*"})
	f([]string{"a*", "b"}, []string{"b*", "abc"}, []string{}, []string{"*"})
}
