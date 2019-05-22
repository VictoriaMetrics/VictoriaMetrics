package storage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func BenchmarkTagFilterMatchSuffix(b *testing.B) {
	b.Run("regexp-any-suffix", func(b *testing.B) {
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
	b.Run("regexp-any-nonzero-suffix", func(b *testing.B) {
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
	b.Run("regexp-special-suffix", func(b *testing.B) {
		key := []byte("foo.*ss?")
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
	b.Run("regexp-or-values", func(b *testing.B) {
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
					logger.Panicf("BUG: unexpected suffix match")
				}
			}
		})
	})
	b.Run("regexp-contains", func(b *testing.B) {
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
					logger.Panicf("BUG: unexpected suffix match")
				}
			}
		})
	})
}
