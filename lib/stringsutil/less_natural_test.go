package stringsutil

import (
	"testing"
)

func TestLessNatural(t *testing.T) {
	f := func(a, b string, resultExpected bool) {
		t.Helper()

		result := LessNatural(a, b)
		if result != resultExpected {
			t.Fatalf("unexpected result for LessNatural(%q, %q); got %v; want %v", a, b, result, resultExpected)
		}
	}

	// comparison with empty string
	f("", "", false)
	f("", "foo", true)
	f("foo", "", false)
	f("", "123", true)
	f("123", "", false)

	// identical values
	f("foo", "foo", false)
	f("123", "123", false)
	f("foo123", "foo123", false)
	f("123foo", "123foo", false)
	f("000", "000", false)
	f("00123", "00123", false)
	f("00foo", "00foo", false)
	f("abc00foo0123", "abc00foo0123", false)

	// identical values with different number of zeroes in front of them
	f("00123", "0123", false)
	f("0123", "00123", true)

	// numeric comparsion
	f("123", "99", false)
	f("99", "123", true)

	// negative numbers (works unexpectedly - this is OK for natural sort order)
	f("-93", "5", false)
	f("5", "-93", true)
	f("-9", "-5", false)
	f("-5", "-9", true)
	f("-93", "foo", true)
	f("foo", "-93", false)
	f("foo-9", "foo-10", true)
	f("foo-10", "foo-9", false)

	// floating-point comparsion (works unexpectedly - this is OK for natural sort order)
	f("1.23", "1.123", true)
	f("1.123", "1.23", false)

	// non-numeric comparison
	f("foo", "bar", false)
	f("fo", "bar", false)
	f("bar", "foo", true)
	f("bar", "fo", true)

	// comparison with common non-numeric prefix
	f("abc_foo", "abc_bar", false)
	f("abc_bar", "abc_foo", true)
	f("abc_foo", "abc_", false)
	f("abc_", "abc_foo", true)
	f("abc_123", "abc_foo", true)
	f("abc_foo", "abc_123", false)

	// comparison with common numeric prefix
	f("123foo", "123bar", false)
	f("123bar", "123foo", true)
	f("123", "123bar", true)
	f("123bar", "123", false)
	f("123_456", "123_78", false)
	f("123_78", "123_456", true)

	// too big integers - fall back to string order
	f("1234567890123456789012345", "1234567890123456789012345", false)
	f("1234567890123456789012345", "123456789012345678901234", false)
	f("123456789012345678901234", "1234567890123456789012345", true)
	f("193456789012345678901234", "1234567890123456789012345", false)
	f("123456789012345678901234", "1934567890123456789012345", true)
	f("1934", "1234567890123456789012345", false)
	f("1234567890123456789012345", "1934", true)

	// integers with many zeroes in front
	f("00000000000000000000000000123", "0000000000000000000000000045", false)
	f("0000000000000000000000000045", "00000000000000000000000000123", true)

	// unicode strings
	f("бвг", "мирг", true)
	f("мирг", "бвг", false)
	f("abcde", "мирг", true)
	f("мирг", "abcde", false)
	f("123", "мирг", true)
	f("мирг", "123", false)
	f("12345", "мирг", true)
	f("мирг", "12345", false)
}
