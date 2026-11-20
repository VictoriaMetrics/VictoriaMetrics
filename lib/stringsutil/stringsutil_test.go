package stringsutil

import (
	"testing"
)

func TestLimitStringLen(t *testing.T) {
	f := func(s string, maxLen int, resultExpected string) {
		t.Helper()

		result := LimitStringLen(s, maxLen)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f("", 1, "")
	f("a", 10, "a")
	f("abc", 2, "abc")
	f("abcd", 3, "abcd")
	f("abcde", 3, "a..e")
	f("abcde", 4, "a..e")
	f("abcde", 5, "abcde")
}

func TestAppendLowercaseToLowercaseFunc(t *testing.T) {
	f := func(s, expected string) {
		t.Helper()

		got := AppendLowercase(nil, s)
		if string(got) != expected {
			t.Fatalf("unexpected result; got %q; want %q", got, expected)
		}

		ToLowercaseFunc(s, func(s string) {
			if s != expected {
				t.Fatalf("unexpected result; got %q; want %q", got, expected)
			}
		})
	}

	// Empty string
	f("", "")

	// ASCII lowercase
	f("hello", "hello")
	f("world", "world")
	f("abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz")

	// ASCII uppercase
	f("HELLO", "hello")
	f("WORLD", "world")
	f("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "abcdefghijklmnopqrstuvwxyz")

	// ASCII mixed case
	f("Hello", "hello")
	f("heLLo", "hello")
	f("WOrld", "world")
	f("HeLLo WoRLd", "hello world")

	// Unicode Cyrillic
	f("привіт", "привіт")
	f("світ", "світ")
	f("ПРИВІТ", "привіт")
	f("СВІТ", "світ")
	f("Привіт", "привіт")
	f("приВіт", "привіт")

	// Unicode Greek
	f("αβγδε", "αβγδε")
	f("ΑΒΓΔΕ", "αβγδε")
	f("Αβγδε", "αβγδε")

	// Latin Extended
	f("café", "café")
	f("naïve", "naïve")
	f("niño", "niño")
	f("ærøå", "ærøå")
	f("ñüöäß", "ñüöäß")
	f("CAFÉ", "café")
	f("NAÏVE", "naïve")
	f("NIÑO", "niño")
	f("ÆRØÅ", "ærøå")
	f("ÑÜÖÄ", "ñüöä")
	f("Café", "café")
	f("naÏve", "naïve")
	f("Niño", "niño")

	// Thai
	f("สวัสดี", "สวัสดี")
	f("โลก", "โลก")

	// Japanese Hiragana
	f("こんにちは", "こんにちは")
	f("せかい", "せかい")

	// Japanese Katakana
	f("コンニチハ", "コンニチハ")
	f("セカイ", "セカイ")

	// Chinese
	f("你好", "你好")
	f("世界", "世界")

	// Devanagari
	f("नमस्ते", "नमस्ते")
	f("दुनिया", "दुनिया")

	// Georgian
	f("გამარჯობა", "გამარჯობა")
	f("ᲒᲐᲛᲐᲠᲯᲝᲑᲐ", "გამარჯობა")

	// Armenian
	f("բարեւ", "բարեւ")
	f("ԲԱՐԵՒ", "բարեւ")

	// Turkish
	f("İSTANBUL", "istanbul")

	// Mixed languages
	f("hello世界", "hello世界")
	f("привет123", "привет123")
	f("test你好", "test你好")
	f("Hello世界", "hello世界")
	f("Привет123", "привет123")
	f("Test你好", "test你好")

	// Emoji and symbols
	f("hello😀world", "hello😀world")
	f("test✨case", "test✨case")
	f("foo🎉bar", "foo🎉bar")
	f("HELLO😀WORLD", "hello😀world")

	// Digits
	f("hello123", "hello123")
	f("test456world", "test456world")
	f("abc123def456", "abc123def456")
	f("123", "123")
	f("456789", "456789")
	f("0", "0")
	f("HELLO123", "hello123")
	f("TEST456WORLD", "test456world")
	f("ABC123DEF456", "abc123def456")

	// Special characters
	f("hello-world", "hello-world")
	f("test_case", "test_case")
	f("foo.bar", "foo.bar")
	f("a@b#c$d", "a@b#c$d")
	f("!@#$%", "!@#$%")
	f(".,;:-_", ".,;:-_")
	f("()[]{}", "()[]{}")
	f("HELLO-WORLD", "hello-world")
	f("TEST_CASE", "test_case")
	f("FOO.BAR", "foo.bar")
	f("A@B#C$D", "a@b#c$d")
}

func TestIsLower(t *testing.T) {
	f := func(s string, want bool) {
		t.Helper()
		if IsLowercase(s) != want {
			t.Fatalf("unexpected result; got %v; want %v for %q", IsLowercase(s), want, s)
		}
	}

	// Empty string
	f("", true)

	// ASCII lowercase
	f("hello", true)
	f("world", true)
	f("abcdefghijklmnopqrstuvwxyz", true)

	// ASCII uppercase
	f("HELLO", false)
	f("WORLD", false)
	f("ABCDEFGHIJKLMNOPQRSTUVWXYZ", false)

	// ASCII mixed case
	f("Hello", false)
	f("heLLo", false)
	f("WOrld", false)

	// Unicode Cyrillic
	f("привіт", true)
	f("світ", true)
	f("ПРИВІТ", false)
	f("СВІТ", false)
	f("Привіт", false)
	f("приВіт", false)

	// Unicode Greek
	f("αβγδε", true)
	f("ΑΒΓΔΕ", false)
	f("Αβγδε", false)

	// Latin Extended with diacritics
	f("café", true)
	f("naïve", true)
	f("niño", true)
	f("ærøå", true)
	f("ñüöäß", true)
	f("CAFÉ", false)
	f("NAÏVE", false)
	f("NIÑO", false)
	f("ÆRØÅ", false)
	f("ÑÜÖÄ", false)
	f("Café", false)
	f("naÏve", false)
	f("Niño", false)

	// Thai
	f("สวัสดี", true)
	f("โลก", true)

	// Japanese Hiragana
	f("こんにちは", true)
	f("せかい", true)

	// Japanese Katakana
	f("コンニチハ", true)
	f("セカイ", true)

	// Chinese characters
	f("你好", true)
	f("世界", true)

	// Devanagari
	f("नमस्ते", true)
	f("दुनिया", true)

	// Georgian
	f("გამარჯობა", true)
	f("ᲒᲐᲛᲐᲠᲯᲝᲑᲐ", false)

	// Armenian
	f("բարեւ", true)
	f("ԲԱՐԵՒ", false)

	// Mixed languages
	f("hello世界", true)
	f("привет123", true)
	f("test你好", true)
	f("Hello世界", false)
	f("Привет123", false)
	f("Test你好", false)

	// Emoji and symbols
	f("hello😀world", true)
	f("test✨case", true)
	f("foo🎉bar", true)

	// Digits
	f("hello123", true)
	f("test456world", true)
	f("abc123def456", true)
	f("123", true)
	f("456789", true)
	f("0", true)
	f("HELLO123", false)
	f("TEST456WORLD", false)
	f("ABC123DEF456", false)

	// Special characters
	f("hello-world", true)
	f("test_case", true)
	f("foo.bar", true)
	f("a@b#c$d", true)
	f("!@#$%", true)
	f(".,;:-_", true)
	f("()[]{}", true)
	f("HELLO-WORLD", false)
	f("TEST_CASE", false)
	f("FOO.BAR", false)
	f("A@B#C$D", false)
}
