package blockcache

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkKeyHashUint64(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var hSum uint64
		var k Key
		for pb.Next() {
			k.Offset++
			h := k.hashUint64()
			hSum += h
		}
		BenchSink.Add(hSum)
	})
}

var BenchSink atomic.Uint64

func BenchmarkCacheGet(b *testing.B) {
	c := NewCache(func() int {
		return 1024 * 1024 * 16
	})
	defer c.MustStop()
	const blocksCount = 10000
	blocks := make([]*testBlock, blocksCount)
	for i := 0; i < blocksCount; i++ {
		blocks[i] = &testBlock{}
		c.PutBlock(Key{Offset: uint64(i)}, blocks[i])
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(blocks)))
	b.RunParallel(func(pb *testing.PB) {
		var k Key
		for pb.Next() {
			for i := 0; i < blocksCount; i++ {
				k.Offset = uint64(i)
				b := c.GetBlock(k)
				if b != blocks[i] {
					panic(fmt.Errorf("unexpected block obtained from the cache; got %v; want %v", b, blocks[i]))
				}
			}
		}
	})
}
