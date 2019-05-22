package mergeset

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func BenchmarkInmemoryBlockMarshal(b *testing.B) {
	const itemsCount = 1000
	var ibSrc inmemoryBlock
	for i := 0; i < itemsCount; i++ {
		item := []byte(fmt.Sprintf("key %d", i))
		if !ibSrc.Add(item) {
			b.Fatalf("cannot add more than %d items", i)
		}
	}
	ibSrc.sort()

	b.ResetTimer()
	b.SetBytes(itemsCount)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var sb storageBlock
		var firstItem, commonPrefix []byte
		var n uint32
		for pb.Next() {
			firstItem, commonPrefix, n, _ = ibSrc.MarshalUnsortedData(&sb, firstItem[:0], commonPrefix[:0], 0)
			if int(n) != itemsCount {
				logger.Panicf("invalid number of items marshaled; got %d; want %d", n, itemsCount)
			}
		}
	})
}

func BenchmarkInmemoryBlockUnmarshal(b *testing.B) {
	var ibSrc inmemoryBlock
	for i := 0; i < 1000; i++ {
		item := []byte(fmt.Sprintf("key %d", i))
		if !ibSrc.Add(item) {
			b.Fatalf("cannot add more than %d items", i)
		}
	}
	var sbSrc storageBlock
	firstItem, commonPrefix, itemsCount, mt := ibSrc.MarshalUnsortedData(&sbSrc, nil, nil, 0)

	b.ResetTimer()
	b.SetBytes(int64(itemsCount))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ib inmemoryBlock
		for pb.Next() {
			if err := ib.UnmarshalData(&sbSrc, firstItem, commonPrefix, itemsCount, mt); err != nil {
				logger.Panicf("cannot unmarshal inmemoryBlock: %s", err)
			}
		}
	})
}
