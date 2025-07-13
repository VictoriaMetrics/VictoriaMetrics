package logstorage

import (
	"sync"

	"github.com/cespare/xxhash/v2"
)

func hash128(data []byte) u128 {
	h := getHasher()
	_, _ = h.Write(data)
	hi := h.Sum64()
	_, _ = h.Write(magicSuffixForHash)
	lo := h.Sum64()
	putHasher(h)

	return u128{
		hi: hi,
		lo: lo,
	}
}

var magicSuffixForHash = []byte("magic!")

func getHasher() *xxhash.Digest {
	v := hasherPool.Get()
	if v == nil {
		return xxhash.New()
	}
	return v.(*xxhash.Digest)
}

func putHasher(h *xxhash.Digest) {
	h.Reset()
	hasherPool.Put(h)
}

var hasherPool sync.Pool
