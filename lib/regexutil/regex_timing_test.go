package regexutil

import (
	"fmt"
	"regexp"
	"testing"
)

func BenchmarkRegexMatchString(b *testing.B) {
	b.Run("unpotimized-noprefix-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "xbar.*|baz", "axbarz", true)
	})
	b.Run("unpotimized-noprefix-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "xbar.*|baz", "zfoobaxz", false)
	})
	b.Run("unpotimized-prefix-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo(bar.*|baz)", "afoobarz", true)
	})
	b.Run("unpotimized-prefix-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo(bar.*|baz)", "zfoobaxz", false)
	})
	b.Run("dot-star-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".*", "foo", true)
	})
	b.Run("dot-plus-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".+", "foo", true)
	})
	b.Run("dot-plus-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".+", "", false)
	})
	b.Run("literal-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo", "afoobar", true)
	})
	b.Run("literal-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo", "abaraa", false)
	})
	b.Run("prefix-dot-star-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo.*", "afoobar", true)
	})
	b.Run("prefix-dot-star-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo.*", "axoobar", false)
	})
	b.Run("prefix-dot-plus-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo.+", "afoobar", true)
	})
	b.Run("prefix-dot-plus-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo.+", "axoobar", false)
	})
	b.Run("or-values-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo|bar|baz", "abaz", true)
	})
	b.Run("or-values-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "foo|bar|baz", "axaz", false)
	})
	b.Run("prefix-or-values-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "x(foo|bar|baz)", "axbaz", true)
	})
	b.Run("prefix-or-values-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "x(foo|bar|baz)", "aabaz", false)
	})
	b.Run("substring-dot-star-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".*foo.*", "afoobar", true)
	})
	b.Run("substring-dot-star-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".*foo.*", "abarbaz", false)
	})
	b.Run("substring-dot-plus-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".+foo.+", "afoobar", true)
	})
	b.Run("substring-dot-plus-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, ".+foo.+", "abarbaz", false)
	})
	b.Run("prefix-substring-dot-star-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "a.*foo.*", "bafoobar", true)
	})
	b.Run("prefix-substring-dot-star-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "a.*foo.*", "babarbaz", false)
	})
	b.Run("prefix-substring-dot-plus-match", func(b *testing.B) {
		benchmarkRegexMatchString(b, "a.+foo.+", "babfoobar", true)
	})
	b.Run("prefix-substring-dot-plus-mismatch", func(b *testing.B) {
		benchmarkRegexMatchString(b, "a.+foo.+", "babarbaz", false)
	})
}

func benchmarkRegexMatchString(b *testing.B, expr, s string, resultExpected bool) {
	r, err := NewRegex(expr)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}
	re := regexp.MustCompile(expr)
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
	b.Run("Regex", func(b *testing.B) {
		f(b, r.MatchString)
	})
	b.Run("StandardRegex", func(b *testing.B) {
		f(b, re.MatchString)
	})
}
