package leveledbytebufferpool

import (
	"math/bits"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pools contains pools for byte slices of various capacities.
//
//    pools[0] is for capacities from 0 to 7
//    pools[1] is for capacities from 8 to 15
//    pools[2] is for capacities from 16 to 31
//    pools[3] is for capacities from 32 to 63
//
var pools [30]sync.Pool

// Get returns byte buffer with the given capacity.
func Get(capacity int) *bytesutil.ByteBuffer {
	for i := 0; i < 2; i++ {
		v := getPool(capacity).Get()
		if v != nil {
			return v.(*bytesutil.ByteBuffer)
		}
		if capacity > 1<<30 {
			break
		}
		capacity *= 2
	}
	return &bytesutil.ByteBuffer{}
}

// Put returns bb to the pool.
func Put(bb *bytesutil.ByteBuffer) {
	capacity := cap(bb.B)
	bb.Reset()
	getPool(capacity).Put(bb)
}

func getPool(size int) *sync.Pool {
	if size < 0 {
		size = 0
	}
	size >>= 3
	n := bits.Len(uint(size))
	if n > len(pools) {
		n = len(pools) - 1
	}
	if n < 0 {
		n = 0
	}
	return &pools[n]
}
