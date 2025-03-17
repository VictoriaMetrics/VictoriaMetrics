package logstorage

import (
	"reflect"
	"testing"
)

func TestFieldsFilter_NilMatch(t *testing.T) {
	var ff *fieldsFilter

	for _, fieldName := range []string{"", "foo"} {
		if ff.match(fieldName) {
			t.Fatalf("unexpected match for %q", fieldName)
		}
	}
}

func TestFieldsFilter_AddMulti(t *testing.T) {
	f := func(filters, expectedFieldNames, expectedWildcards []string) {
		t.Helper()

		var ff fieldsFilter
		ff.addMulti(filters)

		if !reflect.DeepEqual(ff.fieldNames, expectedFieldNames) {
			t.Fatalf("unexpected fieldNames for filters=%#v\ngot\n%#v\nwant\n%#v", filters, ff.fieldNames, expectedFieldNames)
		}
		if !reflect.DeepEqual(ff.wildcards, expectedWildcards) {
			t.Fatalf("unexpected wildcards for filters=%#v\ngot\n%#v\nwant\n%#v", filters, ff.wildcards, expectedWildcards)
		}
	}

	f(nil, nil, nil)
	f([]string{"foo", ""}, []string{"foo", ""}, nil)
	f([]string{"foo*", "bar"}, []string{"bar"}, []string{"foo"})
	f([]string{"foo*", "foo", "bar", "foobar"}, []string{"bar"}, []string{"foo"})
	f([]string{"foo", "foobar", "foo*"}, []string{}, []string{"foo"})
	f([]string{"foobar", "foobar*", "foo*", "bar", "foo", "a*"}, []string{"bar"}, []string{"foo", "a"})
}

func TestFieldsFilter(t *testing.T) {
	f := func(filters []string, fieldName string, resultExpected bool) {
		t.Helper()

		var ff fieldsFilter

		for i := 0; i < 3; i++ {
			ff.addMulti(filters)
			result := ff.match(fieldName)
			if result != resultExpected {
				t.Fatalf("iteration %d: unexpected result for match(%#v, %q); got %v; want %v", i, filters, fieldName, result, resultExpected)
			}
			ff.reset()
		}
	}

	// match against an empty filter
	f(nil, "", false)
	f(nil, "foo", false)

	// match against regular field names
	f([]string{"foo", ""}, "", true)
	f([]string{"foo", ""}, "bar", false)
	f([]string{"foo", ""}, "foo", true)
	f([]string{"foo", ""}, "foobar", false)
	f([]string{"foo", ""}, "barfoo", false)

	// match against wildcards
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "", false)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo", false)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "baz", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "a", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo.qwe", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo.barz", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "bazz", false)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo.bar", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo.abcdef", true)
	f([]string{"a", "foo.qwe", "foo.*", "foo.bar*", "foo.barz", "baz"}, "foo.barzx", true)
}
