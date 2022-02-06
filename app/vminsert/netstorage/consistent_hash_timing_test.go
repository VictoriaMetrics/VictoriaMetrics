package netstorage

import (
	"math/rand"
	"sync/atomic"
	"testing"
)

func BenchmarkConsistentHash(b *testing.B) {
	nodes := []string{
		"node1",
		"node2",
		"node3",
		"node4",
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
		atomic.AddUint64(&BenchSink, uint64(sum))
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

var BenchSink uint64
