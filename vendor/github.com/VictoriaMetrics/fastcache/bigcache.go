package fastcache

import (
	"sync"
	"sync/atomic"

	xxhash "github.com/cespare/xxhash/v2"
)

// maxSubvalueLen is the maximum size of subvalue chunk.
//
// - 16 bytes are for subkey encoding
// - 4 bytes are for len(key)+len(value) encoding inside fastcache
// - 1 byte is implementation detail of fastcache
const maxSubvalueLen = chunkSize - 16 - 4 - 1

// maxKeyLen is the maximum size of key.
//
// - 16 bytes are for (hash + valueLen)
// - 4 bytes are for len(key)+len(subkey)
// - 1 byte is implementation detail of fastcache
const maxKeyLen = chunkSize - 16 - 4 - 1

// SetBig sets (k, v) to c where len(v) may exceed 64KB.
//
// GetBig must be used for reading stored values.
//
// The stored entry may be evicted at any time either due to cache
// overflow or due to unlikely hash collision.
// Pass higher maxBytes value to New if the added items disappear
// frequently.
//
// It is safe to store entries smaller than 64KB with SetBig.
//
// k and v contents may be modified after returning from SetBig.
func (c *Cache) SetBig(k, v []byte) {
	atomic.AddUint64(&c.bigStats.SetBigCalls, 1)
	if len(k) > maxKeyLen {
		atomic.AddUint64(&c.bigStats.TooBigKeyErrors, 1)
		return
	}
	valueLen := len(v)
	valueHash := xxhash.Sum64(v)

	// Split v into chunks with up to 64Kb each.
	subkey := getSubkeyBuf()
	var i uint64
	for len(v) > 0 {
		subkey.B = marshalUint64(subkey.B[:0], valueHash)
		subkey.B = marshalUint64(subkey.B, uint64(i))
		i++
		subvalueLen := maxSubvalueLen
		if len(v) < subvalueLen {
			subvalueLen = len(v)
		}
		subvalue := v[:subvalueLen]
		v = v[subvalueLen:]
		c.Set(subkey.B, subvalue)
	}

	// Write metavalue, which consists of valueHash and valueLen.
	subkey.B = marshalUint64(subkey.B[:0], valueHash)
	subkey.B = marshalUint64(subkey.B, uint64(valueLen))
	c.Set(k, subkey.B)
	putSubkeyBuf(subkey)
}

// GetBig searches for the value for the given k, appends it to dst
// and returns the result.
//
// GetBig returns only values stored via SetBig. It doesn't work
// with values stored via other methods.
//
// k contents may be modified after returning from GetBig.
func (c *Cache) GetBig(dst, k []byte) []byte {
	atomic.AddUint64(&c.bigStats.GetBigCalls, 1)
	subkey := getSubkeyBuf()
	defer putSubkeyBuf(subkey)

	// Read and parse metavalue
	subkey.B = c.Get(subkey.B[:0], k)
	if len(subkey.B) == 0 {
		// Nothing found.
		return dst
	}
	if len(subkey.B) != 16 {
		atomic.AddUint64(&c.bigStats.InvalidMetavalueErrors, 1)
		return dst
	}
	valueHash := unmarshalUint64(subkey.B)
	valueLen := unmarshalUint64(subkey.B[8:])

	// Collect result from chunks.
	dstLen := len(dst)
	if n := dstLen + int(valueLen) - cap(dst); n > 0 {
		dst = append(dst[:cap(dst)], make([]byte, n)...)
	}
	dst = dst[:dstLen]
	var i uint64
	for uint64(len(dst)-dstLen) < valueLen {
		subkey.B = marshalUint64(subkey.B[:0], valueHash)
		subkey.B = marshalUint64(subkey.B, uint64(i))
		i++
		dstNew := c.Get(dst, subkey.B)
		if len(dstNew) == len(dst) {
			// Cannot find subvalue
			return dst[:dstLen]
		}
		dst = dstNew
	}

	// Verify the obtained value.
	v := dst[dstLen:]
	if uint64(len(v)) != valueLen {
		atomic.AddUint64(&c.bigStats.InvalidValueLenErrors, 1)
		return dst[:dstLen]
	}
	h := xxhash.Sum64(v)
	if h != valueHash {
		atomic.AddUint64(&c.bigStats.InvalidValueHashErrors, 1)
		return dst[:dstLen]
	}
	return dst
}

func getSubkeyBuf() *bytesBuf {
	v := subkeyPool.Get()
	if v == nil {
		return &bytesBuf{}
	}
	return v.(*bytesBuf)
}

func putSubkeyBuf(bb *bytesBuf) {
	bb.B = bb.B[:0]
	subkeyPool.Put(bb)
}

var subkeyPool sync.Pool

type bytesBuf struct {
	B []byte
}

func marshalUint64(dst []byte, u uint64) []byte {
	return append(dst, byte(u>>56), byte(u>>48), byte(u>>40), byte(u>>32), byte(u>>24), byte(u>>16), byte(u>>8), byte(u))
}

func unmarshalUint64(src []byte) uint64 {
	_ = src[7]
	return uint64(src[0])<<56 | uint64(src[1])<<48 | uint64(src[2])<<40 | uint64(src[3])<<32 | uint64(src[4])<<24 | uint64(src[5])<<16 | uint64(src[6])<<8 | uint64(src[7])
}
