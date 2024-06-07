package leveledbytebufferpool

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

func TestGetPutConcurrent(t *testing.T) {
	const concurrency = 10
	doneCh := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for capacity := -1; capacity < 100; capacity++ {
				bb := Get(capacity)
				if len(bb.B) > 0 {
					panic(fmt.Errorf("len(bb.B) must be zero; got %d", len(bb.B)))
				}
				if capacity < 0 {
					capacity = 0
				}
				bb.B = slicesutil.SetLength(bb.B, len(bb.B)+capacity)
				Put(bb)
			}
			doneCh <- struct{}{}
		}()
	}
	tc := time.After(10 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case <-tc:
			t.Fatalf("timeout")
		case <-doneCh:
		}
	}
}
