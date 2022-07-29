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

/*
goos: darwin
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkCommonPrefixLen/prefix-len-0-8                 857229081                1.286 ns/op           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-1-8                 709308663                1.473 ns/op     678.75 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-2-8                 666506119                1.763 ns/op    1134.56 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-3-8                 640770340                1.866 ns/op    1607.61 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-4-8                 547492014                2.122 ns/op    1885.26 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-5-8                 471679311                2.300 ns/op    2174.01 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-6-8                 465168498                2.483 ns/op    2416.82 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-7-8                 414648476                2.806 ns/op    2494.37 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-8-8                 551661042                2.135 ns/op    3746.74 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-9-8                 511113198                2.390 ns/op    3765.72 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-33-8                294066612                3.858 ns/op    8553.71 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-67-8                241150149                4.296 ns/op    15595.51 MB/s          0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-127-8               159241046                9.037 ns/op    14053.53 MB/s          0 B/op          0 allocs/op
PASS
*/
func BenchmarkCommonPrefixLen(b *testing.B) {
	for _, prefix := range benchPrefixes {
		b.Run(fmt.Sprintf("prefix-len-%d", len(prefix)), func(b *testing.B) {
			benchmarkCommonPrefixLen(b, prefix)
		})
	}
}

func benchmarkCommonPrefixLen(b *testing.B, prefix string) {
	fmt.Println("prefix is :", prefix)
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

/*
goos: darwin
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkInmemoryBlockMarshal/prefix-len-0-8              231504              5257 ns/op               0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-1-8              220821              4937 ns/op         101.29 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-2-8              236294              4807 ns/op         208.03 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-3-8              246014              4879 ns/op         307.43 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-4-8              187102              5518 ns/op         362.45 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-5-8              207682              6294 ns/op         397.21 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-6-8              157136              6426 ns/op         466.85 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-7-8              191835              6113 ns/op         572.54 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-8-8              183596              5909 ns/op         676.92 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-9-8              219666              5033 ns/op         894.10 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-33-8             225073              5032 ns/op        3278.82 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-67-8             222236              5126 ns/op        6534.83 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockMarshal/prefix-len-127-8            222934              5174 ns/op        12272.12 MB/s          0 B/op          0 allocs/op
PASS
*/
func BenchmarkInmemoryBlockMarshal(b *testing.B) {
	for _, prefix := range benchPrefixes {
		b.Run(fmt.Sprintf("prefix-len-%d", len(prefix)), func(b *testing.B) {
			benchmarkInmemoryBlockMarshal(b, prefix)
		})
	}
}

func benchmarkInmemoryBlockMarshal(b *testing.B, prefix string) {
	const itemsCount = 500
	var ibSrc inmemoryBlock
	for i := 0; i < itemsCount; i++ {
		item := []byte(fmt.Sprintf("%s%d", prefix, i))
		if !ibSrc.Add(item) {
			b.Fatalf("cannot add more than %d items", i)
		}
	}
	sort.Sort(&ibSrc)

	b.ResetTimer()
	b.SetBytes(int64(itemsCount * len(prefix)))
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

/*
goos: darwin
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkInmemoryBlockUnmarshal/prefix-len-0-8            261800              4785 ns/op               0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-1-8            265483              4403 ns/op         113.56 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-2-8            267418              4286 ns/op         233.29 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-3-8            274047              4300 ns/op         348.81 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-4-8            281450              4459 ns/op         448.55 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-5-8            239926              4362 ns/op         573.13 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-6-8            247897              4557 ns/op         658.36 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-7-8            246507              4684 ns/op         747.25 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-8-8            215470              4868 ns/op         821.70 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-9-8            234486              4929 ns/op         912.97 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-33-8           243720              4738 ns/op        3482.74 MB/s           0 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-67-8           164755              6587 ns/op        5085.71 MB/s           2 B/op          0 allocs/op
BenchmarkInmemoryBlockUnmarshal/prefix-len-127-8          167634              6852 ns/op        9266.97 MB/s           3 B/op          0 allocs/op
PASS
*/
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
