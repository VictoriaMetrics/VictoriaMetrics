package storage

import (
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
