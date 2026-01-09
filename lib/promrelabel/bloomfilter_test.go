package promrelabel

import (
	"fmt"
	"testing"
)

func BenchmarkAddTokens(b *testing.B) {
	benchmarks := []struct {
		name       string
		tokenCount int
	}{
		{"10tokens", 10},
		{"50tokens", 50},
		{"100tokens", 100},
		{"500tokens", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			tokens := make([]string, bm.tokenCount)
			for i := range tokens {
				tokens[i] = fmt.Sprintf("label_%d", i)
			}

			bf := &BloomFilter{}
			bf.EnsureSize(bm.tokenCount * 2)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				bf.Reset()
				bf.AddTokens(tokens)
			}
		})
	}
}

func BenchmarkAddTokensRealisticLabels(b *testing.B) {
	tokens := []string{
		"__name__",
		"job",
		"instance",
		"prometheus",
		"__address__",
		"__scheme__",
		"__metrics_path__",
		"__param_target",
		"environment",
		"region",
		"datacenter",
		"service",
		"version",
		"cluster",
		"namespace",
		"pod",
		"container",
		"node",
		"monitoring",
		"http://localhost:9090/metrics",
	}

	bf := &BloomFilter{}
	bf.EnsureSize(len(tokens) * 2)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bf.Reset()
		bf.AddTokens(tokens)
	}
}

func BenchmarkContainsAll(b *testing.B) {
	benchmarks := []struct {
		name       string
		tokenCount int
	}{
		{"10tokens", 10},
		{"50tokens", 50},
		{"100tokens", 100},
		{"500tokens", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			tokens := make([]string, bm.tokenCount)
			for i := range tokens {
				tokens[i] = fmt.Sprintf("label_%d", i)
			}

			bf := &BloomFilter{}
			bf.EnsureSize(bm.tokenCount * 2)
			bf.AddTokens(tokens)

			hashes := AppendTokensHashes(nil, tokens)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = bf.ContainsAll(hashes)
			}
		})
	}
}

func BenchmarkContainsAllMiss(b *testing.B) {
	populateTokens := make([]string, 100)
	for i := range populateTokens {
		populateTokens[i] = fmt.Sprintf("populate_%d", i)
	}

	bf := &BloomFilter{}
	bf.EnsureSize(len(populateTokens) * 2)
	bf.AddTokens(populateTokens)

	checkTokens := make([]string, 100)
	for i := range checkTokens {
		checkTokens[i] = fmt.Sprintf("check_%d", i)
	}
	hashes := AppendTokensHashes(nil, checkTokens)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = bf.ContainsAll(hashes)
	}
}
