package logstorage

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// bloomFilterHashesCount is the number of different hashes to use for bloom filter.
const bloomFilterHashesCount = 6

// bloomFilterBitsPerItem is the number of bits to use per each token.
const bloomFilterBitsPerItem = 16

// bloomFilterMarshal appends marshaled bloom filter for tokens to dst and returns the result.
func bloomFilterMarshal(dst []byte, tokens []string) []byte {
	bf := getBloomFilter()
	bf.mustInit(tokens)
	dst = bf.marshal(dst)
	putBloomFilter(bf)
	return dst
}

type bloomFilter struct {
	bits []uint64
}

func (bf *bloomFilter) reset() {
	clear(bf.bits)
	bf.bits = bf.bits[:0]
}

// marshal appends marshaled bf to dst and returns the result.
func (bf *bloomFilter) marshal(dst []byte) []byte {
	bits := bf.bits
	for _, word := range bits {
		dst = encoding.MarshalUint64(dst, word)
	}
	return dst
}

// unmarshal unmarshals bf from src.
func (bf *bloomFilter) unmarshal(src []byte) error {
	if len(src)%8 != 0 {
		return fmt.Errorf("cannot unmarshal bloomFilter from src with size not multiple by 8; len(src)=%d", len(src))
	}
	bf.reset()
	wordsCount := len(src) / 8
	bits := slicesutil.SetLength(bf.bits, wordsCount)
	for i := range bits {
		bits[i] = encoding.UnmarshalUint64(src)
		src = src[8:]
	}
	bf.bits = bits
	return nil
}

// mustInit initializes bf with the given tokens
func (bf *bloomFilter) mustInit(tokens []string) {
	bitsCount := len(tokens) * bloomFilterBitsPerItem
	wordsCount := (bitsCount + 63) / 64
	bits := slicesutil.SetLength(bf.bits, wordsCount)
	bloomFilterAdd(bits, tokens)
	bf.bits = bits
}

// bloomFilterAdd adds the given tokens to the bloom filter bits
func bloomFilterAdd(bits []uint64, tokens []string) {
	maxBits := uint64(len(bits)) * 64
	var buf [8]byte
	hp := (*uint64)(unsafe.Pointer(&buf[0]))
	for _, token := range tokens {
		*hp = xxhash.Sum64(bytesutil.ToUnsafeBytes(token))
		for i := 0; i < bloomFilterHashesCount; i++ {
			hi := xxhash.Sum64(buf[:])
			(*hp)++
			idx := hi % maxBits
			i := idx / 64
			j := idx % 64
			mask := uint64(1) << j
			w := bits[i]
			if (w & mask) == 0 {
				bits[i] = w | mask
			}
		}
	}
}

// containsAll returns true if bf contains all the given tokens.
func (bf *bloomFilter) containsAll(tokens []string) bool {
	bits := bf.bits
	if len(bits) == 0 {
		return true
	}
	maxBits := uint64(len(bits)) * 64
	var buf [8]byte
	hp := (*uint64)(unsafe.Pointer(&buf[0]))
	for _, token := range tokens {
		*hp = xxhash.Sum64(bytesutil.ToUnsafeBytes(token))
		for i := 0; i < bloomFilterHashesCount; i++ {
			hi := xxhash.Sum64(buf[:])
			(*hp)++
			idx := hi % maxBits
			i := idx / 64
			j := idx % 64
			mask := uint64(1) << j
			w := bits[i]
			if (w & mask) == 0 {
				// The token is missing
				return false
			}
		}
	}
	return true
}

// containsAny returns true if bf contains at least a single token from the given tokens.
func (bf *bloomFilter) containsAny(tokens []string) bool {
	bits := bf.bits
	if len(bits) == 0 {
		return true
	}
	maxBits := uint64(len(bits)) * 64
	var buf [8]byte
	hp := (*uint64)(unsafe.Pointer(&buf[0]))
nextToken:
	for _, token := range tokens {
		*hp = xxhash.Sum64(bytesutil.ToUnsafeBytes(token))
		for i := 0; i < bloomFilterHashesCount; i++ {
			hi := xxhash.Sum64(buf[:])
			(*hp)++
			idx := hi % maxBits
			i := idx / 64
			j := idx % 64
			mask := uint64(1) << j
			w := bits[i]
			if (w & mask) == 0 {
				// The token is missing. Check the next token
				continue nextToken
			}
		}
		// It is likely the token exists in the bloom filter
		return true
	}
	return false
}

func getBloomFilter() *bloomFilter {
	v := bloomFilterPool.Get()
	if v == nil {
		return &bloomFilter{}
	}
	return v.(*bloomFilter)
}

func putBloomFilter(bf *bloomFilter) {
	bf.reset()
	bloomFilterPool.Put(bf)
}

var bloomFilterPool sync.Pool
