package commonprefixlen

import (
	"unsafe"
)

//go:nosplit
//go:noescape
//goland:noinspection GoUnusedParameter
func __common_prefix_len(a uintptr, lenA int, b uintptr, lenB int) (ret int)

func CommonPrefixLen(a, b []byte) int {
	return __common_prefix_len(
		uintptr(unsafe.Pointer(&a[0])),
		len(a),
		uintptr(unsafe.Pointer(&b[0])),
		len(b),
	)
}

/*
cpu: Intel(R) Core(TM) i7-6700HQ CPU @ 2.60GHz
BenchmarkCommonPrefixLen/prefix-len-0-8                 1000000000               0.6302 ns/op          0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-1-8                 1000000000               0.9141 ns/op   1093.99 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-2-8                 907274526                1.600 ns/op    1250.15 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-3-8                 617327667                1.672 ns/op    1794.07 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-4-8                 705519706                1.457 ns/op    2744.52 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-5-8                 698212174                1.602 ns/op    3121.34 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-6-8                 659606611                1.642 ns/op    3654.40 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-7-8                 666251821                1.689 ns/op    4144.46 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-8-8                 644601556                1.820 ns/op    4394.68 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-9-8                 607549078                1.955 ns/op    4604.42 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-33-8                209053814                5.609 ns/op    5883.79 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-67-8                111235797               10.45 ns/op     6409.00 MB/s           0 B/op          0 allocs/op
BenchmarkCommonPrefixLen/prefix-len-127-8               48990189                22.48 ns/op     5648.81 MB/s           0 B/op          0 allocs/op
*/
func commonPrefixLenOneByOne(a, b []byte) int {
	i := 0
	if len(a) > len(b) {
		for i < len(b) && a[i] == b[i] {
			i++
		}
	} else {
		for i < len(a) && a[i] == b[i] {
			i++
		}
	}
	return i
}
