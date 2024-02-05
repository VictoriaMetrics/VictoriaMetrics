package bloomfilter

import (
	"sync/atomic"
	"unsafe"

	"github.com/cespare/xxhash/v2"
)

const hashesCount = 4
const bitsPerItem = 16

// Filter count uniq elements.
//
// It is safe using the Filter from concurrent goroutines.
type Filter struct {
	maxItems int
	bits     []uint64
}

// NewFilter creates new Filter, which can hold up to maxItems unique items.
func NewFilter(maxItems int) *Filter {
	bitsCount := maxItems * bitsPerItem
	bits := make([]uint64, (bitsCount+63)/64)
	return &Filter{
		maxItems: maxItems,
		bits:     bits,
	}
}

// Reset resets f to initial state.
//
// It is expected no other goroutines call f methods during Reset call.
func (f *Filter) Reset() {
	bits := f.bits
	for i := range bits {
		bits[i] = 0
	}
}

// Has checks whether h presents in f.
//
// Has can be called from concurrent goroutines.
func (f *Filter) Has(h uint64) bool {
	bits := f.bits
	maxBits := uint64(len(bits)) * 64
	bp := (*[8]byte)(unsafe.Pointer(&h))
	b := bp[:]
	for i := 0; i < hashesCount; i++ {
		hi := xxhash.Sum64(b)
		h++
		idx := hi % maxBits
		i := idx / 64
		j := idx % 64
		mask := uint64(1) << j
		w := atomic.LoadUint64(&bits[i])
		if (w & mask) == 0 {
			return false
		}
	}
	return true
}

// Add adds h to f.
//
// True is returned if h was missing in f.
//
// Add can be called from concurrent goroutines.
// If the same h is added to f from concurrent goroutines, then both goroutines may return true.
func (f *Filter) Add(h uint64) bool {
	bits := f.bits
	maxBits := uint64(len(bits)) * 64
	bp := (*[8]byte)(unsafe.Pointer(&h))
	b := bp[:]
	isNew := false
	for i := 0; i < hashesCount; i++ {
		hi := xxhash.Sum64(b)
		h++
		idx := hi % maxBits
		i := idx / 64
		j := idx % 64
		mask := uint64(1) << j
		w := atomic.LoadUint64(&bits[i])
		for (w & mask) == 0 {
			wNew := w | mask
			// The wNew != w most of the time, so there is no need in using atomic.LoadUint64
			// in front of atomic.CompareAndSwapUint64 in order to try avoiding slow inter-CPU synchronization.
			if atomic.CompareAndSwapUint64(&bits[i], w, wNew) {
				isNew = true
				break
			}
			w = atomic.LoadUint64(&bits[i])
		}
	}
	return isNew
}
