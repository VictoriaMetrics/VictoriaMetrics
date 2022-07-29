package commonprefixlen

import (
	"fmt"
	"testing"
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
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset/commonprefixlen
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkCommonPrefixLen/prefix-len-0-8                 450079596                2.867 ns/op           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-1-8                 391974652                2.611 ns/op     382.95 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-2-8                 439879129                2.704 ns/op     739.51 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-3-8                 426323844                2.921 ns/op    1027.11 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-4-8                 362720601                3.136 ns/op    1275.46 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-5-8                 382358844                3.349 ns/op    1492.95 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-6-8                 361422925                3.432 ns/op    1748.37 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-7-8                 341047308                3.638 ns/op    1924.06 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-8-8                 328263012                3.974 ns/op    2013.05 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-9-8                 292529353                3.861 ns/op    2330.87 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-33-8                80806491                12.87 ns/op     2563.41 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-67-8                105599992               11.46 ns/op     5844.85 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-127-8               47549875                22.11 ns/op     5743.06 MB/s           0 B/op          0 allocs/op
PASS
*/

/*
第二版  内存对齐的加载
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkCommonPrefixLen/prefix-len-0-8                 563243301                1.940 ns/op           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-1-8                 537914098                2.076 ns/op     481.61 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-2-8                 445219232                2.705 ns/op     739.46 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-3-8                 462200941                2.886 ns/op    1039.46 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-4-8                 380611388                3.630 ns/op    1101.93 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-5-8                 328284748                3.882 ns/op    1287.89 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-6-8                 296094052                3.830 ns/op    1566.63 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-7-8                 284616625                4.093 ns/op    1710.35 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-8-8                 341884394                3.680 ns/op    2173.63 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-9-8                 332640072                3.845 ns/op    2340.44 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-33-8                128675481                9.340 ns/op    3533.37 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-67-8                71399806                16.11 ns/op     4158.29 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-127-8               35636883                30.61 ns/op     4148.44 MB/s           0 B/op          0 allocs/op
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
	//fmt.Println("prefix is :", prefix)
	b.ReportAllocs()
	b.SetBytes(int64(len(prefix)))
	b.RunParallel(func(pb *testing.PB) {
		a := append([]byte{}, prefix...)
		a = append(a, 'a')
		b := append([]byte{}, prefix...)
		b = append(b, 'b')
		for pb.Next() {
			n := CommonPrefixLen(a, b)
			//n := commonPrefixLenOneByOne(a, b)
			if n != len(prefix) {
				panic(fmt.Errorf("unexpected prefix len; got %d; want %d", n, len(prefix)))
			}
		}
	})
}
