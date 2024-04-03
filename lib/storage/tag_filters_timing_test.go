package storage

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func BenchmarkTagFilterMatchSuffix(b *testing.B) {
	b.Run("regexp-any-suffix-match", func(b *testing.B) {
		key := []byte("foo.*")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected suffix mismatch")
				}
			}
		})
	})
	b.Run("regexp-any-suffix-match-anchored", func(b *testing.B) {
		key := []byte("^foo.*$")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected suffix mismatch")
				}
			}
		})
	})
	b.Run("regexp-any-nonzero-suffix-match", func(b *testing.B) {
		key := []byte("foo.+")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected suffix mismatch")
				}
			}
		})
	})
	b.Run("regexp-any-nonzero-suffix-match-anchored", func(b *testing.B) {
		key := []byte("^foo.+$")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected suffix mismatch")
				}
			}
		})
	})
	b.Run("regexp-any-nonzero-suffix-mismatch", func(b *testing.B) {
		key := []byte("foo.+")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte(""))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected suffix match")
				}
			}
		})
	})
	b.Run("regexp-special-suffix-match", func(b *testing.B) {
		key := []byte("foo.*sss?")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfdss"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected suffix mismatch")
				}
			}
		})
	})
	b.Run("regexp-special-suffix-mismatch", func(b *testing.B) {
		key := []byte("foo.*sss?")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected suffix match")
				}
			}
		})
	})
	b.Run("regexp-or-values-match", func(b *testing.B) {
		key := []byte("foo|bar|baz")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("bar"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected mismatch")
				}
			}
		})
	})
	b.Run("regexp-or-values-mismatch", func(b *testing.B) {
		key := []byte("foo|bar|baz")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected match")
				}
			}
		})
	})
	b.Run("regexp-contains-dot-star-match", func(b *testing.B) {
		key := []byte(".*foo.*")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksfoodfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected mismatch")
				}
			}
		})
	})
	b.Run("regexp-contains-dot-star-mismatch", func(b *testing.B) {
		key := []byte(".*foo.*")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected match")
				}
			}
		})
	})
	b.Run("regexp-contains-dot-plus-match", func(b *testing.B) {
		key := []byte(".+foo.+")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfoofds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected mismatch")
				}
			}
		})
	})
	b.Run("regexp-contains-dot-plus-mismatch", func(b *testing.B) {
		key := []byte(".+foo.+")
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("ojksdfds"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected match")
				}
			}
		})
	})
	b.Run("regexp-graphite-metric-mismatch", func(b *testing.B) {
		key := []byte(`foo[^.]*?\.bar\.baz\.[^.]*?\.ddd`)
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("foo1.xar.baz.sss.ddd"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if ok {
					logger.Panicf("BUG: unexpected match")
				}
			}
		})
	})
	b.Run("regexp-graphite-metric-match", func(b *testing.B) {
		key := []byte(`foo[^.]*?\.bar\.baz\.[^.]*?\.ddd`)
		isNegative := false
		isRegexp := true
		suffix := marshalTagValue(nil, []byte("foo1.bar.baz.sss.ddd"))
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			var tf tagFilter
			if err := tf.Init(nil, nil, key, isNegative, isRegexp); err != nil {
				logger.Panicf("BUG: unexpected error: %s", err)
			}
			for pb.Next() {
				ok, err := tf.matchSuffix(suffix)
				if err != nil {
					logger.Panicf("BUG: unexpected error: %s", err)
				}
				if !ok {
					logger.Panicf("BUG: unexpected mismatch")
				}
			}
		})
	})
}

// Run the following command to get the execution cost of all matches
//
// go test -run=none -bench=BenchmarkOptimizedReMatchCost -count 20 github.com/VictoriaMetrics/VictoriaMetrics/lib/storage | tee cost.txt
// benchstat ./cost.txt
//
// Calculate the multiplier of default for each match overhead.

func BenchmarkOptimizedReMatchCost(b *testing.B) {
	b.Run("fullMatchCost", func(b *testing.B) {
		reMatch := func(b []byte) bool {
			return len(b) == 0
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run("literalMatchCost", func(b *testing.B) {
		s := "foo1.bar.baz.sss.ddd"
		reMatch := func(b []byte) bool {
			return string(b) == s
		}
		suffix := []byte(s)
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run("threeLiteralsMatchCost", func(b *testing.B) {
		s := []string{"foo", "bar", "baz"}
		reMatch := func(b []byte) bool {
			for _, v := range s {
				if string(b) == v {
					return true
				}
			}
			return false
		}
		suffix := []byte("ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".*", func(b *testing.B) {
		reMatch := func(_ []byte) bool {
			return true
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".+", func(b *testing.B) {
		reMatch := func(b []byte) bool {
			return len(b) > 0
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run("prefix.*", func(b *testing.B) {
		s := []byte("foo1.bar")
		reMatch := func(b []byte) bool {
			return bytes.HasPrefix(b, s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run("prefix.+", func(b *testing.B) {
		s := []byte("foo1.bar")
		reMatch := func(b []byte) bool {
			return len(b) > len(s) && bytes.HasPrefix(b, s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".*suffix", func(b *testing.B) {
		s := []byte("sss.ddd")
		reMatch := func(b []byte) bool {
			return bytes.HasSuffix(b, s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".+suffix", func(b *testing.B) {
		s := []byte("sss.ddd")
		reMatch := func(b []byte) bool {
			return len(b) > len(s) && bytes.HasSuffix(b[1:], s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".*middle.*", func(b *testing.B) {
		s := []byte("bar.baz")
		reMatch := func(b []byte) bool {
			return bytes.Contains(b, s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".*middle.+", func(b *testing.B) {
		s := []byte("bar.baz")
		reMatch := func(b []byte) bool {
			return len(b) > len(s) && bytes.Contains(b[:len(b)-1], s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".+middle.*", func(b *testing.B) {
		s := []byte("bar.baz")
		reMatch := func(b []byte) bool {
			return len(b) > len(s) && bytes.Contains(b[1:], s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run(".+middle.+", func(b *testing.B) {
		s := []byte("bar.baz")
		reMatch := func(b []byte) bool {
			return len(b) > len(s)+1 && bytes.Contains(b[1:len(b)-1], s)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
	b.Run("reMatchCost", func(b *testing.B) {
		re := regexp.MustCompile(`foo[^.]*?\.bar\.baz\.[^.]*?\.ddd`)
		reMatch := func(b []byte) bool {
			return re.Match(b)
		}
		suffix := []byte("foo1.bar.baz.sss.ddd")
		b.ReportAllocs()
		b.SetBytes(int64(1))
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				reMatch(suffix)
			}
		})
	})
}
