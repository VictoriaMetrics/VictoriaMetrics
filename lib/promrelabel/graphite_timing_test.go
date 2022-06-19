package promrelabel

import (
	"fmt"
	"testing"
)

func BenchmarkGraphiteMatchTemplateMatch(b *testing.B) {
	b.Run("match-short", func(b *testing.B) {
		tpl := "*.bar.baz"
		s := "foo.bar.baz"
		benchmarkGraphiteMatchTemplateMatch(b, tpl, s, true)
	})
	b.Run("mismtach-short", func(b *testing.B) {
		tpl := "*.bar.baz"
		s := "foo.aaa"
		benchmarkGraphiteMatchTemplateMatch(b, tpl, s, false)
	})
	b.Run("match-long", func(b *testing.B) {
		tpl := "*.*.*.bar.*.baz"
		s := "foo.bar.baz.bar.aa.baz"
		benchmarkGraphiteMatchTemplateMatch(b, tpl, s, true)
	})
	b.Run("mismatch-long", func(b *testing.B) {
		tpl := "*.*.*.bar.*.baz"
		s := "foo.bar.baz.bar.aa.bb"
		benchmarkGraphiteMatchTemplateMatch(b, tpl, s, false)
	})
}

func benchmarkGraphiteMatchTemplateMatch(b *testing.B, tpl, s string, okExpected bool) {
	gmt := newGraphiteMatchTemplate(tpl)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var matches []string
		for pb.Next() {
			var ok bool
			matches, ok = gmt.Match(matches[:0], s)
			if ok != okExpected {
				panic(fmt.Errorf("unexpected ok=%v for tpl=%q, s=%q", ok, tpl, s))
			}
		}
	})
}

func BenchmarkGraphiteReplaceTemplateExpand(b *testing.B) {
	b.Run("one-replacement", func(b *testing.B) {
		tpl := "$1"
		matches := []string{"", "foo"}
		resultExpected := "foo"
		benchmarkGraphiteReplaceTemplateExpand(b, tpl, matches, resultExpected)
	})
	b.Run("one-replacement-with-prefix", func(b *testing.B) {
		tpl := "x-$1"
		matches := []string{"", "foo"}
		resultExpected := "x-foo"
		benchmarkGraphiteReplaceTemplateExpand(b, tpl, matches, resultExpected)
	})
	b.Run("one-replacement-with-prefix-suffix", func(b *testing.B) {
		tpl := "x-$1-y"
		matches := []string{"", "foo"}
		resultExpected := "x-foo-y"
		benchmarkGraphiteReplaceTemplateExpand(b, tpl, matches, resultExpected)
	})
	b.Run("two-replacements", func(b *testing.B) {
		tpl := "$1$2"
		matches := []string{"", "foo", "bar"}
		resultExpected := "foobar"
		benchmarkGraphiteReplaceTemplateExpand(b, tpl, matches, resultExpected)
	})
	b.Run("two-replacements-with-delimiter", func(b *testing.B) {
		tpl := "$1-$2"
		matches := []string{"", "foo", "bar"}
		resultExpected := "foo-bar"
		benchmarkGraphiteReplaceTemplateExpand(b, tpl, matches, resultExpected)
	})
}

func benchmarkGraphiteReplaceTemplateExpand(b *testing.B, tpl string, matches []string, resultExpected string) {
	grt := newGraphiteReplaceTemplate(tpl)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var b []byte
		for pb.Next() {
			b = grt.Expand(b[:0], matches)
			if string(b) != resultExpected {
				panic(fmt.Errorf("unexpected result; got\n%q\nwant\n%q", b, resultExpected))
			}
		}
	})
}
