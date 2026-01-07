package ce

import (
	"math/rand"
	"testing"

	"github.com/axiomhq/hyperloglog"
	"github.com/stretchr/testify/assert"
)

func Benchmark_Estimate(b *testing.B) {
	setup := func(precision uint8, b *testing.B) *hyperloglog.Sketch {
		hll, err := hyperloglog.NewSketch(precision, true)
		assert.NoError(b, err)

		for range 1_000_000 {
			hll.InsertHash(rand.Uint64())
		}

		return hll
	}

	b.Run("p=8", func(b *testing.B) {
		hll8 := setup(8, b)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = hll8.Estimate()
		}

		b.StopTimer()

	})

	b.Run("p=10", func(b *testing.B) {
		hll10 := setup(10, b)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = hll10.Estimate()
		}

		b.StopTimer()
	})

	b.Run("p=12", func(b *testing.B) {
		hll12 := setup(12, b)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = hll12.Estimate()
		}

		b.StopTimer()
	})

	b.Run("p=14", func(b *testing.B) {
		hll14 := setup(14, b)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_ = hll14.Estimate()
		}

		b.StopTimer()
	})
}
