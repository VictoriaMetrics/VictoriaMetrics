package jump

import (
	"hash"
	"hash/crc32"
	"hash/crc64"
	"hash/fnv"
	"io"
)

// Hash takes a 64 bit key and the number of buckets. It outputs a bucket
// number in the range [0, buckets).
// If the number of buckets is less than or equal to 0 then one 1 is used.
func Hash(key uint64, buckets int32) int32 {
	var b, j int64

	if buckets <= 0 {
		buckets = 1
	}

	for j < int64(buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(b+1) * (float64(int64(1)<<31) / float64((key>>33)+1)))
	}

	return int32(b)
}

// HashString takes string as key instead of an int and uses a KeyHasher to
// generate a key compatible with Hash().
func HashString(key string, buckets int32, h KeyHasher) int32 {
	h.Reset()
	_, err := io.WriteString(h, key)
	if err != nil {
		panic(err)
	}
	return Hash(h.Sum64(), buckets)
}

// KeyHasher is a subset of hash.Hash64 in the standard library.
type KeyHasher interface {
	// Write (via the embedded io.Writer interface) adds more data to the
	// running hash.
	// It never returns an error.
	io.Writer

	// Reset resets the KeyHasher to its initial state.
	Reset()

	// Return the result of the added bytes (via io.Writer).
	Sum64() uint64
}

// Hasher represents a jump consistent hasher using a string as key.
type Hasher struct {
	n int32
	h KeyHasher
}

// New returns a new instance of of Hasher.
func New(n int, h KeyHasher) *Hasher {
	return &Hasher{int32(n), h}
}

// N returns the number of buckets the hasher can assign to.
func (h *Hasher) N() int {
	return int(h.n)
}

// Hash returns the integer hash for the given key.
func (h *Hasher) Hash(key string) int {
	return int(HashString(key, h.n, h.h))
}

// KeyHashers available in the standard library for use with HashString() and Hasher.
var (
	// CRC32 uses the 32-bit Cyclic Redundancy Check (CRC-32) with the IEEE
	// polynomial.
	NewCRC32 func() hash.Hash64 = func() hash.Hash64 { return &crc32Hasher{crc32.NewIEEE()} }
	// CRC64 uses the 64-bit Cyclic Redundancy Check (CRC-64) with the ECMA
	// polynomial.
	NewCRC64 func() hash.Hash64 = func() hash.Hash64 { return crc64.New(crc64.MakeTable(crc64.ECMA)) }
	// FNV1 uses the non-cryptographic hash function FNV-1.
	NewFNV1 func() hash.Hash64 = func() hash.Hash64 { return fnv.New64() }
	// FNV1a uses the non-cryptographic hash function FNV-1a.
	NewFNV1a func() hash.Hash64 = func() hash.Hash64 { return fnv.New64a() }

	// These are deprecated because they're not safe for concurrent use. Please
	// use the New* functions instead.
	CRC32 hash.Hash64 = &crc32Hasher{crc32.NewIEEE()}
	CRC64 hash.Hash64 = crc64.New(crc64.MakeTable(crc64.ECMA))
	FNV1  hash.Hash64 = fnv.New64()
	FNV1a hash.Hash64 = fnv.New64a()
)
