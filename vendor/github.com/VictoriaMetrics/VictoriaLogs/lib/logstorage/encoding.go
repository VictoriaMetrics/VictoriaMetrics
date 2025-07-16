package logstorage

import (
	"fmt"
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// marshalStringsBlock marshals a and appends the result to dst.
//
// The marshaled strings block can be unmarshaled with stringsBlockUnmarshaler.
func marshalStringsBlock(dst []byte, a []string) []byte {
	// Encode string lengths
	u64s := encoding.GetUint64s(len(a))
	aLens := u64s.A
	totalLen := 0
	for i, s := range a {
		aLens[i] = uint64(len(s))
		totalLen += len(s)
	}
	dst = marshalUint64Block(dst, aLens)
	encoding.PutUint64s(u64s)

	// Encode strings
	if areConstValues(a) {
		// Special case for const values
		dst = marshalBytesBlock(dst, bytesutil.ToUnsafeBytes(a[0]))
	} else {
		// Regular case for non-const values
		bb := bbPool.Get()

		// Pre-allocate the needed memory in order to reduce the number of reallocations in the loop below.
		b := slicesutil.SetLength(bb.B, totalLen)

		b = b[:0]
		for _, s := range a {
			b = append(b, s...)
		}
		dst = marshalBytesBlock(dst, b)

		bb.B = b
		bbPool.Put(bb)
	}

	return dst
}

// stringsBlockUnmarshaler is used for unmarshaling the block returned from marshalStringsBlock()
//
// use getStringsBlockUnmarshaler() for obtaining the unmarshaler from the pool in order to save memory allocations.
type stringsBlockUnmarshaler struct {
	// data contains the data for the unmarshaled values
	data []byte
}

func (sbu *stringsBlockUnmarshaler) reset() {
	sbu.data = sbu.data[:0]
}

func (sbu *stringsBlockUnmarshaler) copyString(s string) string {
	dataLen := len(sbu.data)
	sbu.data = append(sbu.data, s...)
	return bytesutil.ToUnsafeString(sbu.data[dataLen:])
}

func (sbu *stringsBlockUnmarshaler) appendFields(dst, src []Field) []Field {
	for _, f := range src {
		dst = append(dst, Field{
			Name:  sbu.copyString(f.Name),
			Value: sbu.copyString(f.Value),
		})
	}
	return dst
}

// unmarshal unmarshals itemsCount strings from src, appends them to dst and returns the result.
//
// The returned strings are valid until sbu.reset() call.
func (sbu *stringsBlockUnmarshaler) unmarshal(dst []string, src []byte, itemsCount uint64) ([]string, error) {
	u64s := encoding.GetUint64s(0)
	defer encoding.PutUint64s(u64s)

	// Decode string lengths
	var tail []byte
	var err error
	u64s.A, tail, err = unmarshalUint64Block(u64s.A[:0], src, itemsCount)
	if err != nil {
		return dst, fmt.Errorf("cannot unmarshal string lengths: %w", err)
	}
	aLens := u64s.A
	src = tail

	// Read bytes block into sbu.data
	dataLen := len(sbu.data)
	sbu.data, tail, err = unmarshalBytesBlock(sbu.data, src)
	if err != nil {
		return dst, fmt.Errorf("cannot unmarshal bytes block with strings: %w", err)
	}
	if len(tail) > 0 {
		return dst, fmt.Errorf("unexpected non-empty tail after reading bytes block with strings; len(tail)=%d", len(tail))
	}

	// Decode strings from sbu.data into dst
	data := sbu.data[dataLen:]

	dst = slicesutil.SetLength(dst, len(dst)+len(aLens))
	dstA := dst[len(dst)-len(aLens):]

	if len(aLens) >= 2 && areConstUint64s(aLens) && uint64(len(data)) == aLens[0] {
		// Special case - decode a constant string
		s := bytesutil.ToUnsafeString(data)
		for i := range dstA {
			dstA[i] = s
		}
		return dst, nil
	}

	for i := range dstA {
		sLen := aLens[i]
		if uint64(len(data)) < sLen {
			return dst, fmt.Errorf("cannot unmarshal a string with the length %d bytes from %d bytes", sLen, len(data))
		}
		s := bytesutil.ToUnsafeString(data[:sLen])
		data = data[sLen:]
		dstA[i] = s
	}

	return dst, nil
}

func areConstUint64s(a []uint64) bool {
	if len(a) == 0 {
		return false
	}
	v := a[0]
	for i := 1; i < len(a); i++ {
		if v != a[i] {
			return false
		}
	}
	return true
}

// marshalUint64Block appends marshaled a to dst and returns the result.
func marshalUint64Block(dst []byte, a []uint64) []byte {
	bb := bbPool.Get()
	bb.B = marshalUint64Items(bb.B[:0], a)
	dst = marshalBytesBlock(dst, bb.B)
	bbPool.Put(bb)
	return dst
}

// unmarshalUint64Block appends unmarshaled from src itemsCount uint64 items to dst and returns the result.
func unmarshalUint64Block(dst []uint64, src []byte, itemsCount uint64) ([]uint64, []byte, error) {
	bb := bbPool.Get()
	defer bbPool.Put(bb)

	// Unmarshal the underlying bytes block
	var err error
	bb.B, src, err = unmarshalBytesBlock(bb.B[:0], src)
	if err != nil {
		return dst, src, fmt.Errorf("cannot unmarshal bytes block: %w", err)
	}

	// Unmarshal the items from bb.
	dst, err = unmarshalUint64Items(dst, bb.B, itemsCount)
	if err != nil {
		return dst, src, fmt.Errorf("cannot unmarshal %d uint64 items from bytes block of length %d bytes: %w", itemsCount, len(bb.B), err)
	}
	return dst, src, nil
}

const (
	uintBlockType8  = 0
	uintBlockType16 = 1
	uintBlockType32 = 2
	uintBlockType64 = 3

	uintBlockTypeConst8  = 4
	uintBlockTypeConst16 = 5
	uintBlockTypeConst32 = 6
	uintBlockTypeConst64 = 7
)

// marshalUint64Items appends the marshaled a items to dst and returns the result.
func marshalUint64Items(dst []byte, a []uint64) []byte {
	// Do not marshal len(a), since it is expected that unmarshaler knows it.

	nMax := uint64(0)
	for _, n := range a {
		if n > nMax {
			nMax = n
		}
	}
	areConsts := len(a) >= 2 && areConstUint64s(a)
	switch {
	case nMax < (1 << 8):
		if areConsts {
			dst = append(dst, uintBlockTypeConst8)
			dst = append(dst, byte(a[0]))
		} else {
			dst = append(dst, uintBlockType8)
			for _, n := range a {
				dst = append(dst, byte(n))
			}
		}
	case nMax < (1 << 16):
		if areConsts {
			dst = append(dst, uintBlockTypeConst16)
			dst = encoding.MarshalUint16(dst, uint16(a[0]))
		} else {
			dst = append(dst, uintBlockType16)
			for _, n := range a {
				dst = encoding.MarshalUint16(dst, uint16(n))
			}
		}
	case nMax < (1 << 32):
		if areConsts {
			dst = append(dst, uintBlockTypeConst32)
			dst = encoding.MarshalUint32(dst, uint32(a[0]))
		} else {
			dst = append(dst, uintBlockType32)
			for _, n := range a {
				dst = encoding.MarshalUint32(dst, uint32(n))
			}
		}
	default:
		if areConsts {
			dst = append(dst, uintBlockTypeConst64)
			dst = encoding.MarshalUint64(dst, a[0])
		} else {
			dst = append(dst, uintBlockType64)
			for _, n := range a {
				dst = encoding.MarshalUint64(dst, n)
			}
		}
	}
	return dst
}

// unmarshalUint64Items appends unmarshaled from src itemsCount uint64 items to dst and returns the result.
func unmarshalUint64Items(dst []uint64, src []byte, itemsCount uint64) ([]uint64, error) {
	// Unmarshal block type
	if len(src) < 1 {
		return dst, fmt.Errorf("cannot unmarshal uint64 block type from empty src")
	}
	blockType := src[0]
	src = src[1:]

	dstLen := uint64(len(dst)) + itemsCount
	if dstLen > math.MaxInt {
		return dst, fmt.Errorf("too long destination buffer: len=%d; must not exceed %d", dstLen, uint64(math.MaxInt))
	}
	dst = slicesutil.SetLength(dst, int(dstLen))
	dstA := dst[dstLen-itemsCount:]

	switch blockType {
	case uintBlockType8:
		// A block with items smaller than 1<<8 bytes
		if uint64(len(src)) != itemsCount {
			return dst, fmt.Errorf("unexpected block length for %d uint8 items; got %d bytes; want %d bytes", itemsCount, len(src), itemsCount)
		}
		for i := range dstA {
			dstA[i] = uint64(src[i])
		}
	case uintBlockType16:
		// A block with items smaller than 1<<16 bytes
		if uint64(len(src)) != 2*itemsCount {
			return dst, fmt.Errorf("unexpected block length for %d uint16 items; got %d bytes; want %d bytes", itemsCount, len(src), 2*itemsCount)
		}
		for i := range dstA {
			idx := 2 * i
			v := encoding.UnmarshalUint16(src[idx : idx+2])
			dst[i] = uint64(v)
		}
	case uintBlockType32:
		// A block with items smaller than 1<<32 bytes
		if uint64(len(src)) != 4*itemsCount {
			return dst, fmt.Errorf("unexpected block length for %d uint32 items; got %d bytes; want %d bytes", itemsCount, len(src), 4*itemsCount)
		}
		for i := range dstA {
			idx := 4 * i
			v := encoding.UnmarshalUint32(src[idx : idx+4])
			dst[i] = uint64(v)
		}
	case uintBlockType64:
		// A block with items smaller than 1<<64 bytes
		if uint64(len(src)) != 8*itemsCount {
			return dst, fmt.Errorf("unexpected block length for %d uint64 items; got %d bytes; want %d bytes", itemsCount, len(src), 8*itemsCount)
		}
		for i := range dstA {
			idx := 8 * i
			v := encoding.UnmarshalUint64(src[idx : idx+8])
			dst[i] = v
		}
	case uintBlockTypeConst8:
		if len(src) != 1 {
			return dst, fmt.Errorf("unexpected block length for const uint8 item; got %d bytes; want 1 byte", len(src))
		}
		v := uint64(src[0])
		for i := range dstA {
			dst[i] = v
		}
	case uintBlockTypeConst16:
		if len(src) != 2 {
			return dst, fmt.Errorf("unexpected block length for const uint16 item; got %d bytes; want 2 bytes", len(src))
		}
		v := uint64(encoding.UnmarshalUint16(src))
		for i := range dstA {
			dst[i] = v
		}
	case uintBlockTypeConst32:
		if len(src) != 4 {
			return dst, fmt.Errorf("unexpected block length for const uint32 item; got %d bytes; want 4 bytes", len(src))
		}
		v := uint64(encoding.UnmarshalUint32(src))
		for i := range dstA {
			dst[i] = v
		}
	case uintBlockTypeConst64:
		if len(src) != 8 {
			return dst, fmt.Errorf("unexpected block length for const uint64 item; got %d bytes; want 8 bytes", len(src))
		}
		v := encoding.UnmarshalUint64(src)
		for i := range dstA {
			dst[i] = v
		}
	default:
		return dst, fmt.Errorf("unexpected uint64 block type: %d", blockType)
	}
	return dst, nil
}

const (
	marshalBytesTypePlain = 0
	marshalBytesTypeZSTD  = 1
)

func marshalBytesBlock(dst, src []byte) []byte {
	if len(src) < 128 {
		// Marshal the block in plain without compression
		dst = append(dst, marshalBytesTypePlain)
		dst = append(dst, byte(len(src)))
		return append(dst, src...)
	}

	// Compress the block
	dst = append(dst, marshalBytesTypeZSTD)
	compressLevel := getCompressLevel(len(src))
	bb := bbPool.Get()
	bb.B = encoding.CompressZSTDLevel(bb.B[:0], src, compressLevel)
	dst = encoding.MarshalVarUint64(dst, uint64(len(bb.B)))
	dst = append(dst, bb.B...)
	bbPool.Put(bb)
	return dst
}

func getCompressLevel(dataLen int) int {
	if dataLen <= 512 {
		return 1
	}
	if dataLen <= 4*1024 {
		return 2
	}
	return 3
}

func unmarshalBytesBlock(dst, src []byte) ([]byte, []byte, error) {
	if len(src) < 1 {
		return dst, src, fmt.Errorf("cannot unmarshal block type from empty src")
	}
	blockType := src[0]
	src = src[1:]
	switch blockType {
	case marshalBytesTypePlain:
		// Plain block

		// Read block length
		if len(src) < 1 {
			return dst, src, fmt.Errorf("cannot unmarshal plain block size from empty src")
		}
		blockLen := int(src[0])
		src = src[1:]
		if len(src) < blockLen {
			return dst, src, fmt.Errorf("cannot read plain block with the size %d bytes from %b bytes", blockLen, len(src))
		}

		// Copy the block to dst
		dst = append(dst, src[:blockLen]...)
		src = src[blockLen:]
		return dst, src, nil
	case marshalBytesTypeZSTD:
		// Compressed block

		// Read block length
		blockLen, nSize := encoding.UnmarshalVarUint64(src)
		if nSize <= 0 {
			return dst, src, fmt.Errorf("cannot unmarshal compressed block size")
		}
		src = src[nSize:]
		if uint64(len(src)) < blockLen {
			return dst, src, fmt.Errorf("cannot read compressed block with the size %d bytes from %d bytes", blockLen, len(src))
		}
		compressedBlock := src[:blockLen]
		src = src[blockLen:]

		// Decompress the block
		bb := bbPool.Get()
		var err error
		bb.B, err = encoding.DecompressZSTD(bb.B[:0], compressedBlock)
		if err != nil {
			return dst, src, fmt.Errorf("cannot decompress block: %w", err)
		}

		// Copy the decompressed block to dst.
		dst = append(dst, bb.B...)
		bbPool.Put(bb)
		return dst, src, nil
	default:
		return dst, src, fmt.Errorf("unexpected block type: %d; supported types: 0, 1", blockType)
	}
}

var bbPool bytesutil.ByteBufferPool

// getStringsBlockUnmarshaler returns stringsBlockUnmarshaler from the pool.
//
// Return back the stringsBlockUnmarshaler to the pool by calling putStringsBlockUnmarshaler().
func getStringsBlockUnmarshaler() *stringsBlockUnmarshaler {
	v := sbuPool.Get()
	if v == nil {
		return &stringsBlockUnmarshaler{}
	}
	return v.(*stringsBlockUnmarshaler)
}

// putStringsBlockUnmarshaler returns back sbu to the pool.
//
// sbu mustn't be used after returning to the pool.
func putStringsBlockUnmarshaler(sbu *stringsBlockUnmarshaler) {
	sbu.reset()
	sbuPool.Put(sbu)
}

var sbuPool sync.Pool
