package storage

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

const benchBlockRows = 8192

// benchTimestamps returns timestamps with the given scrape interval and up to jitterMs of jitter.
//
// Zero jitter makes Block marshal them as DeltaConst, which doesn't need zstd.
func benchTimestamps(seed int64, rows int, jitterMs int) []int64 {
	rng := rand.New(rand.NewSource(seed))
	a := make([]int64, rows)
	for i := range a {
		a[i] = 1_700_000_000_000 + int64(i)*15000
		if jitterMs > 0 {
			a[i] += int64(rng.Intn(jitterMs + 1))
		}
	}
	return a
}

// benchValues returns counter-like (integer increments) or gauge-like (float random walk) values.
func benchValues(seed int64, rows int, counter bool) []float64 {
	rng := rand.New(rand.NewSource(seed))
	a := make([]float64, rows)
	v := 1000.0
	for i := range a {
		if counter {
			v += float64(rng.Intn(50))
		} else {
			v += rng.NormFloat64() * 0.37
		}
		a[i] = v
	}
	return a
}

// benchMarshaledBlock returns a marshaled block, which is ready for UnmarshalBlock.
func benchMarshaledBlock(timestamps []int64, values []float64) []byte {
	mantissas, scale := decimal.AppendFloatToDecimal(nil, values)

	var src Block
	src.Init(&TSID{}, timestamps, mantissas, scale, 64)
	src.MarshalData(0, 0)
	return MarshalBlock(nil, &src)
}

type benchCase struct {
	name     string
	jitterMs int
	counter  bool
}

var benchCases = []benchCase{
	{"const-ts/gauge", 0, false},
	{"jitter-ts/gauge", 20, false},
	{"jitter-ts/counter", 20, true},
}

func BenchmarkBlockUnmarshalData(b *testing.B) {
	for _, c := range benchCases {
		b.Run(c.name, func(b *testing.B) {
			data := benchMarshaledBlock(benchTimestamps(1, benchBlockRows, c.jitterMs), benchValues(1, benchBlockRows, c.counter))

			b.ReportAllocs()
			b.SetBytes(int64(benchBlockRows * 16))
			b.ResetTimer()

			var blk Block
			for i := 0; i < b.N; i++ {
				blk.Reset()
				if _, err := UnmarshalBlock(&blk, data); err != nil {
					b.Fatalf("UnmarshalBlock: %s", err)
				}
				if err := blk.UnmarshalData(); err != nil {
					b.Fatalf("UnmarshalData: %s", err)
				}
			}
		})
	}
}

// BenchmarkBlockUnmarshalDataParallel checks that nothing scales badly with the number of goroutines.
func BenchmarkBlockUnmarshalDataParallel(b *testing.B) {
	data := benchMarshaledBlock(benchTimestamps(1, benchBlockRows, 20), benchValues(1, benchBlockRows, true))

	b.ReportAllocs()
	b.SetBytes(int64(benchBlockRows * 16))
	b.RunParallel(func(pb *testing.PB) {
		var blk Block
		for pb.Next() {
			blk.Reset()
			if _, err := UnmarshalBlock(&blk, data); err != nil {
				panic(fmt.Errorf("UnmarshalBlock: %w", err))
			}
			if err := blk.UnmarshalData(); err != nil {
				panic(fmt.Errorf("UnmarshalData: %w", err))
			}
		}
	})
}
