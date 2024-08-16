package logstorage

import (
	"fmt"
	"testing"
)

func BenchmarkMatchAnyCasePrefix(b *testing.B) {
	b.Run("match-ascii-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "err", []string{"error here", "another error here", "foo bar baz error"}, true)
	})
	b.Run("match-ascii-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "err", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, true)
	})
	b.Run("match-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "err", []string{"error здесь", "another error здесь", "тест bar baz error"}, true)
	})
	b.Run("match-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "err", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, true)
	})

	b.Run("mismatch-partial-ascii-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "rror", []string{"error here", "another error here", "foo bar baz error"}, false)
	})
	b.Run("mismatch-partial-ascii-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "rror", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, false)
	})
	b.Run("mismatch-partial-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "rror", []string{"error здесь", "another error здесь", "тест bar baz error"}, false)
	})
	b.Run("mismatch-partial-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "rror", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, false)
	})

	b.Run("mismatch-full-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "warning", []string{"error here", "another error here", "foo bar baz error"}, false)
	})
	b.Run("mismatch-full-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "warning", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, false)
	})
	b.Run("mismatch-full-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "warning", []string{"error здесь", "another error здесь", "тест bar baz error"}, false)
	})
	b.Run("mismatch-full-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePrefix(b, "warning", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, false)
	})
}

func benchmarkMatchAnyCasePrefix(b *testing.B, phraseLowercase string, a []string, resultExpected bool) {
	n := 0
	for _, s := range a {
		n += len(s)
	}

	b.ReportAllocs()
	b.SetBytes(int64(n))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, s := range a {
				result := matchAnyCasePrefix(s, phraseLowercase)
				if result != resultExpected {
					panic(fmt.Errorf("unexpected result for matchAnyCasePrefix(%q, %q); got %v; want %v", s, phraseLowercase, result, resultExpected))
				}
			}
		}
	})
}

func BenchmarkMatchAnyCasePhrase(b *testing.B) {
	b.Run("match-ascii-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "error", []string{"error here", "another error here", "foo bar baz error"}, true)
	})
	b.Run("match-ascii-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "error", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, true)
	})
	b.Run("match-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "error", []string{"error здесь", "another error здесь", "тест bar baz error"}, true)
	})
	b.Run("match-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "error", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, true)
	})

	b.Run("mismatch-partial-ascii-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "rror", []string{"error here", "another error here", "foo bar baz error"}, false)
	})
	b.Run("mismatch-partial-ascii-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "rror", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, false)
	})
	b.Run("mismatch-partial-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "rror", []string{"error здесь", "another error здесь", "тест bar baz error"}, false)
	})
	b.Run("mismatch-partial-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "rror", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, false)
	})

	b.Run("mismatch-full-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "warning", []string{"error here", "another error here", "foo bar baz error"}, false)
	})
	b.Run("mismatch-full-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "warning", []string{"Error here", "another eRROr here", "foo BAR Baz error"}, false)
	})
	b.Run("mismatch-full-unicode-lowercase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "warning", []string{"error здесь", "another error здесь", "тест bar baz error"}, false)
	})
	b.Run("mismatch-full-unicode-mixcase", func(b *testing.B) {
		benchmarkMatchAnyCasePhrase(b, "warning", []string{"error Здесь", "another Error здесь", "тEст bar baz ErRor"}, false)
	})
}

func benchmarkMatchAnyCasePhrase(b *testing.B, phraseLowercase string, a []string, resultExpected bool) {
	n := 0
	for _, s := range a {
		n += len(s)
	}

	b.ReportAllocs()
	b.SetBytes(int64(n))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, s := range a {
				result := matchAnyCasePhrase(s, phraseLowercase)
				if result != resultExpected {
					panic(fmt.Errorf("unexpected result for matchAnyCasePhrase(%q, %q); got %v; want %v", s, phraseLowercase, result, resultExpected))
				}
			}
		}
	})
}
