package mergeset

import (
	"fmt"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var benchPrefixes = []string{
	"", "x", "xy", "xyz", "xyz1", "xyz12",
	"xyz123", "xyz1234", "01234567", "xyz123456", "xyz123456789012345678901234567890",
	"aljkljfdpjopoewpoirerop934093094poipdfidpfdsfkjljdfpjoejkdjfljpfdkl",
	"aljkljfdpjopoewpoirerop934093094poipdfidpfdsfkjljdfpjoejkdjfljpfdkllkj321oiiou321oijlkfdfjjlfdsjdslkfjdslfjldskafjldsflkfdsjlkj",
}

func BenchmarkCommonPrefixLen(b *testing.B) {
	for _, prefix := range benchPrefixes {
		b.Run(fmt.Sprintf("prefix-len-%d", len(prefix)), func(b *testing.B) {
			benchmarkCommonPrefixLen(b, prefix)
		})
	}
}

func benchmarkCommonPrefixLen(b *testing.B, prefix string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(prefix)))
	b.RunParallel(func(pb *testing.PB) {
		a := append([]byte{}, prefix...)
		a = append(a, 'a')
		b := append([]byte{}, prefix...)
		b = append(b, 'b')
		for pb.Next() {
			n := commonPrefixLen(a, b)
			if n != len(prefix) {
				panic(fmt.Errorf("unexpected prefix len; got %d; want %d", n, len(prefix)))
			}
		}
	})
}

func BenchmarkInmemoryBlockMarshal(b *testing.B) {
	for _, prefix := range benchPrefixes {
		b.Run(fmt.Sprintf("prefix-len-%d", len(prefix)), func(b *testing.B) {
			benchmarkInmemoryBlockMarshal(b, prefix)
		})
	}
}

func benchmarkInmemoryBlockMarshal(b *testing.B, prefix string) {
	const itemsCount = 500

	b.ResetTimer()
	b.SetBytes(int64(itemsCount * len(prefix)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ibSrc inmemoryBlock
		for i := 0; i < itemsCount; i++ {
			item := []byte(fmt.Sprintf("%s%d", prefix, i))
			if !ibSrc.Add(item) {
				b.Fatalf("cannot add more than %d items", i)
			}
		}
		sort.Sort(&ibSrc)

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
	for _, prefix := range benchPrefixes {
		b.Run(fmt.Sprintf("prefix-len-%d", len(prefix)), func(b *testing.B) {
			benchmarkInmemoryBlockUnmarshal(b, prefix)
		})
	}
}

func benchmarkInmemoryBlockUnmarshal(b *testing.B, prefix string) {
	var ibSrc inmemoryBlock
	for i := 0; i < 500; i++ {
		item := []byte(fmt.Sprintf("%s%d", prefix, i))
		if !ibSrc.Add(item) {
			b.Fatalf("cannot add more than %d items", i)
		}
	}
	var sbSrc storageBlock
	firstItem, commonPrefix, itemsCount, mt := ibSrc.MarshalUnsortedData(&sbSrc, nil, nil, 0)

	b.ResetTimer()
	b.SetBytes(int64(itemsCount) * int64(len(prefix)))
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
