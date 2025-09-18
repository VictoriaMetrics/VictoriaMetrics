package tenantmetrics

import (
	"runtime"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
)

func BenchmarkCounterMapGrowth(b *testing.B) {
	f := func(b *testing.B, numTenants uint32, nProcs int) {
		b.Helper()

		for i := 0; i < b.N; i++ {
			cm := NewCounterMap("foobar")
			var wg sync.WaitGroup
			for range nProcs {
				wg.Add(1)
				go func() {
					for i := range numTenants {
						cm.Get(&auth.Token{AccountID: i, ProjectID: i}).Inc()
					}
					wg.Done()
				}()
			}
			wg.Wait()
		}
	}

	b.Run("n=100,nProcs=GOMAXPROCS", func(b *testing.B) {
		f(b, 100, runtime.GOMAXPROCS(0))
	})

	b.Run("n=100,nProcs=2", func(b *testing.B) {
		f(b, 100, 2)
	})

	b.Run("n=1000,nProcs=2", func(b *testing.B) {
		f(b, 1000, 2)
	})

	b.Run("n=10000,nProcs=2", func(b *testing.B) {
		f(b, 10000, 2)
	})
}
