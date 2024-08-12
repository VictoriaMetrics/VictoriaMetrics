package leveledbytebufferpool

import (
	"math/bits"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pools contains pools for byte slices of various capacities.
//
//	pools[0] is for capacities from 0 to 64
//	pools[1] is for capacities from 65 to 128
//	pools[2] is for capacities from 129 to 256
//	...
//	pools[n] is for capacities from 2^(n+5)+1 to 2^(n+6)
//
// Limit the maximum capacity to 2^18, since there are no performance benefits
// in caching byte slices with bigger capacities.
var pools [12]sync.Pool

// Get returns byte buffer with the given capacity.
func Get(capacity int) *bytesutil.ByteBuffer {
	id, capacityNeeded := getPoolIDAndCapacity(capacity)
	for i := 0; i < 2; i++ {
		if id < 0 || id >= len(pools) {
			break
		}
		if v := pools[id].Get(); v != nil {
			return v.(*bytesutil.ByteBuffer)
		}
		id++
	}
	return &bytesutil.ByteBuffer{
		B: make([]byte, 0, capacityNeeded),
	}
}

// Put returns bb to the pool.
func Put(bb *bytesutil.ByteBuffer) {
	capacity := cap(bb.B)
	id, poolCapacity := getPoolIDAndCapacity(capacity)
	if capacity <= poolCapacity {
		bb.Reset()
		pools[id].Put(bb)
	}
}

func getPoolIDAndCapacity(size int) (int, int) {
	size--
	if size < 0 {
		size = 0
	}
	size >>= 6
	id := bits.Len(uint(size))
	if id >= len(pools) {
		id = len(pools) - 1
	}
	return id, (1 << (id + 6))
}
