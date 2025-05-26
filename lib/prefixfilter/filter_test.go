package prefixfilter

import (
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

func TestFilter_MatchString(t *testing.T) {
	f := func(allow, deny []string, s string, resultExpected bool) {
		t.Helper()

		f := GetFilter()
		defer PutFilter(f)

		f.AddAllowFilters(allow)
		f.AddDenyFilters(deny)

		result := f.MatchString(s)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f(nil, nil, "", false)
	f(nil, nil, "foo", false)

	// Allow all
	f([]string{"*"}, nil, "", true)
	f([]string{"*"}, nil, "foo", true)
	f([]string{"*", "a", "b*"}, nil, "", true)
	f([]string{"*", "a", "b*"}, nil, "foo", true)

	// Allow all, deny some
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "foo", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bar", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "baz", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bam", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "bamp", false)
	f([]string{"*"}, []string{"foo", "ba*", "baz*", "bam"}, "abc", true)

	// Deny all
	f([]string{"*"}, []string{"*"}, "", false)
	f([]string{"*"}, []string{"*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"*"}, "abc", false)

	// Allow some
	f([]string{"foo", "ba*"}, nil, "", false)
	f([]string{"foo", "ba*"}, nil, "foo", true)
	f([]string{"foo", "ba*"}, nil, "foobar", false)
	f([]string{"foo", "ba*"}, nil, "ba", true)
	f([]string{"foo", "ba*"}, nil, "bar", true)
	f([]string{"foo", "ba*"}, nil, "abc", false)

	// Mix allow / deny
	f([]string{"foo", "ba*"}, []string{"bar"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"bar"}, "foo", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"bar"}, "baz", true)
	f([]string{"foo", "ba*"}, []string{"bar"}, "barz", true)

	// Deny overrides everything
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "foo", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "ba", false)
	f([]string{"foo", "ba*"}, []string{"b*", "f*"}, "bar", false)

	// Deny overrides some
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "abc", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "foo", true)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "bar", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "baz", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "barz", false)
	f([]string{"foo", "ba*"}, []string{"bar*", "baz*"}, "bam", true)

	// Deny equals allow
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "foo", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "bar", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "abc", false)
	f([]string{"foo", "bar"}, []string{"foo", "bar"}, "", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "foo", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "bar", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "abc", false)
	f([]string{"foo", "bar*"}, []string{"foo", "bar*"}, "", false)
}
