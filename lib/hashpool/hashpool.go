package hashpool

import (
	"github.com/cespare/xxhash/v2"
	"sync"
)

var xxhashPool = &sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

// Get return a *xxhash.Digest from hash pool.
func Get() *xxhash.Digest {
	return xxhashPool.Get().(*xxhash.Digest)
}

// Put a *xxhash.Digest back to the hash pool.
func Put(x *xxhash.Digest) {
	xxhashPool.Put(x)
}
