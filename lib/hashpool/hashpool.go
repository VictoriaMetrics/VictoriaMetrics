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

func Get() *xxhash.Digest {
	return xxhashPool.Get().(*xxhash.Digest)
}

func Put(x *xxhash.Digest) {
	xxhashPool.Put(x)
	return
}
