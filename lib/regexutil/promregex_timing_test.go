package regexutil

import (
	"fmt"
	"regexp"
	"testing"
)

func BenchmarkPromRegexMatchString(b *testing.B) {
	b.Run("unpotimized-noprefix-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "xbar.*|baz", "xbarz", true)
	})
	b.Run("unpotimized-noprefix-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "xbar.*|baz", "zfoobarz", false)
	})
	b.Run("unpotimized-prefix-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo(bar.*|baz)", "foobarz", true)
	})
	b.Run("unpotimized-prefix-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo(bar.*|baz)", "zfoobarz", false)
	})
	b.Run("dot-star-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".*", "foo", true)
	})
	b.Run("dot-plus-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".+", "foo", true)
	})
	b.Run("dot-plus-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".+", "", false)
	})
	b.Run("literal-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo", "foo", true)
	})
	b.Run("literal-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo", "bar", false)
	})
	b.Run("prefix-dot-star-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo.*", "foobar", true)
	})
	b.Run("prefix-dot-star-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo.*", "afoobar", false)
	})
	b.Run("prefix-dot-plus-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo.+", "foobar", true)
	})
	b.Run("prefix-dot-plus-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo.+", "afoobar", false)
	})
	b.Run("or-values-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo|bar|baz", "baz", true)
	})
	b.Run("or-values-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "foo|bar|baz", "abaz", false)
	})
	b.Run("prefix-or-values-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "x(foo|bar|baz)", "xbaz", true)
	})
	b.Run("prefix-or-values-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "x(foo|bar|baz)", "abaz", false)
	})
	b.Run("substring-dot-star-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".*foo.*", "afoobar", true)
	})
	b.Run("substring-dot-star-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".*foo.*", "abarbaz", false)
	})
	b.Run("substring-dot-plus-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".+foo.+", "afoobar", true)
	})
	b.Run("substring-dot-plus-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, ".+foo.+", "abarbaz", false)
	})
	b.Run("prefix-substring-dot-star-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "a.*foo.*", "afoobar", true)
	})
	b.Run("prefix-substring-dot-star-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "a.*foo.*", "abarbaz", false)
	})
	b.Run("prefix-substring-dot-plus-match", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "a.+foo.+", "abfoobar", true)
	})
	b.Run("prefix-substring-dot-plus-mismatch", func(b *testing.B) {
		benchmarkPromRegexMatchString(b, "a.+foo.+", "abarbaz", false)
	})
}

func benchmarkPromRegexMatchString(b *testing.B, expr, s string, resultExpected bool) {
	pr, err := NewPromRegex(expr)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}
	re := regexp.MustCompile("^(?:" + expr + ")$")
	f := func(b *testing.B, matchString func(s string) bool) {
		b.SetBytes(1)
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				result := matchString(s)
				if result != resultExpected {
					panic(fmt.Errorf("unexpected result when matching %s against %s; got %v; want %v", s, expr, result, resultExpected))
				}
			}
		})
	}
	b.Run("PromRegex", func(b *testing.B) {
		f(b, pr.MatchString)
	})
	b.Run("StandardRegex", func(b *testing.B) {
		f(b, re.MatchString)
	})
}
