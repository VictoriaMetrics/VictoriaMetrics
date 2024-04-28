package stringsutil

import (
	"strings"
	"sync/atomic"
	"testing"
)

func BenchmarkAppendLowercase(b *testing.B) {
	b.Run("ascii-all-lowercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"foo bar baz abc def", "23k umlkds", "lq, poweri2349)"})
	})
	b.Run("ascii-some-uppercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"Foo Bar baz ABC def", "23k umlKDs", "lq, Poweri2349)"})
	})
	b.Run("ascii-all-uppercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"FOO BAR BAZ ABC DEF", "23K UMLKDS", "LQ, POWERI2349)"})
	})
	b.Run("unicode-all-lowercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"хщцукодл длобючф дл", "23и юбывлц", "лф, длощшу2349)"})
	})
	b.Run("unicode-some-uppercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"Хщцукодл Длобючф ДЛ", "23и юбыВЛц", "лф, Длощшу2349)"})
	})
	b.Run("unicode-all-uppercase", func(b *testing.B) {
		benchmarkAppendLowercase(b, []string{"ХЩЦУКОДЛ ДЛОБЮЧФ ДЛ", "23И ЮБЫВЛЦ", "ЛФ, ДЛОЩШУ2349)"})
	})
}

func benchmarkAppendLowercase(b *testing.B, a []string) {
	n := 0
	for _, s := range a {
		n += len(s)
	}

	b.ReportAllocs()
	b.SetBytes(int64(n))
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		var n uint64
		for pb.Next() {
			buf = buf[:0]
			for _, s := range a {
				buf = AppendLowercase(buf, s)
			}
			n += uint64(len(buf))
		}
		GlobalSink.Add(n)
	})
}

func BenchmarkStringsToLower(b *testing.B) {
	b.Run("ascii-all-lowercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"foo bar baz abc def", "23k umlkds", "lq, poweri2349)"})
	})
	b.Run("ascii-some-uppercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"Foo Bar baz ABC def", "23k umlKDs", "lq, Poweri2349)"})
	})
	b.Run("ascii-all-uppercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"FOO BAR BAZ ABC DEF", "23K UMLKDS", "LQ, POWERI2349)"})
	})
	b.Run("unicode-all-lowercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"хщцукодл длобючф дл", "23и юбывлц", "лф, длощшу2349)"})
	})
	b.Run("unicode-some-uppercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"Хщцукодл Длобючф ДЛ", "23и юбыВЛц", "лф, Длощшу2349)"})
	})
	b.Run("unicode-all-uppercase", func(b *testing.B) {
		benchmarkStringsToLower(b, []string{"ХЩЦУКОДЛ ДЛОБЮЧФ ДЛ", "23И ЮБЫВЛЦ", "ЛФ, ДЛОЩШУ2349)"})
	})
}

func benchmarkStringsToLower(b *testing.B, a []string) {
	n := 0
	for _, s := range a {
		n += len(s)
	}

	b.ReportAllocs()
	b.SetBytes(int64(n))
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		var n uint64
		for pb.Next() {
			buf = buf[:0]
			for _, s := range a {
				sLower := strings.ToLower(s)
				buf = append(buf, sLower...)
			}
			n += uint64(len(buf))
		}
		GlobalSink.Add(n)
	})
}

var GlobalSink atomic.Uint64
