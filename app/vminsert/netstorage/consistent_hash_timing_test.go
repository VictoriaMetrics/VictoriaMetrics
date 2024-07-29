package netstorage

import (
	"math/rand"
	"sync/atomic"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func BenchmarkConsistentHash(b *testing.B) {
	nodes := []uint64{
		xxhash.Sum64String("node1"),
		xxhash.Sum64String("node2"),
		xxhash.Sum64String("node3"),
		xxhash.Sum64String("node4"),
	}
	rh := newConsistentHash(nodes, 0)

	b.ReportAllocs()
	b.SetBytes(int64(len(benchKeys)))
	b.RunParallel(func(pb *testing.PB) {
		sum := 0
		for pb.Next() {
			for _, k := range benchKeys {
				idx := rh.getNodeIdx(k, nil)
				sum += idx
			}
		}
		BenchSink.Add(uint64(sum))
	})
}

var benchKeys = func() []uint64 {
	r := rand.New(rand.NewSource(1))
	keys := make([]uint64, 10000)
	for i := 0; i < len(keys); i++ {
		keys[i] = r.Uint64()
	}
	return keys
}()

var BenchSink atomic.Uint64
