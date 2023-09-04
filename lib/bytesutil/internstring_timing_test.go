package bytesutil

import (
	"fmt"
	"testing"
)

func BenchmarkInternString(b *testing.B) {
	a := make([]string, 10000)
	for i := range a {
		a[i] = fmt.Sprintf("string_%d", i)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, s := range a {
				sResult := InternString(s)
				if sResult != s {
					panic(fmt.Sprintf("unexpected string obtained; got %q; want %q", sResult, s))
				}
			}
		}
	})
}
