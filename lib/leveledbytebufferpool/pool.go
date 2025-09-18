package leveledbytebufferpool

import (
	"math/bits"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// pools contains pools for byte slices of various capacities.
//
//	pools[0] is for capacities from 0 to 256
//	pools[1] is for capacities from 257 to 512
//	pools[2] is for capacities from 513 to 1024
//	...
//	pools[n] is for capacities from 2^(n+7)+1 to 2^(n+8)
//
// Limit the maximum capacity to 2^18, since there are no performance benefits
// in caching byte slices with bigger capacities.
var pools [10]sync.Pool

// Get returns byte buffer, which is able to store at least dataLen bytes.
func Get(dataLen int) *bytesutil.ByteBuffer {
	id, capacityNeeded := getPoolIDAndCapacity(dataLen)
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
	size >>= 8
	id := bits.Len(uint(size))
	if id >= len(pools) {
		id = len(pools) - 1
	}
	return id, (1 << (id + 8))
}
