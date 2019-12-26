package gozstd

/*
#cgo CFLAGS: -O3

#define ZSTD_STATIC_LINKING_ONLY
#include "zstd.h"
#include "zstd_errors.h"

#include <stdint.h>  // for uintptr_t

// The following *_wrapper functions allow avoiding memory allocations
// durting calls from Go.
// See https://github.com/golang/go/issues/24450 .

static size_t ZSTD_compressCCtx_wrapper(ZSTD_CCtx* ctx, uintptr_t dst, size_t dstCapacity, uintptr_t src, size_t srcSize, int compressionLevel) {
    return ZSTD_compressCCtx(ctx, (void*)dst, dstCapacity, (const void*)src, srcSize, compressionLevel);
}

static size_t ZSTD_compress_usingCDict_wrapper(ZSTD_CCtx* ctx, uintptr_t dst, size_t dstCapacity, uintptr_t src, size_t srcSize, const ZSTD_CDict* cdict) {
    return ZSTD_compress_usingCDict(ctx, (void*)dst, dstCapacity, (const void*)src, srcSize, cdict);
}

static size_t ZSTD_decompressDCtx_wrapper(ZSTD_DCtx* ctx, uintptr_t dst, size_t dstCapacity, uintptr_t src, size_t srcSize) {
    return ZSTD_decompressDCtx(ctx, (void*)dst, dstCapacity, (const void*)src, srcSize);
}

static size_t ZSTD_decompress_usingDDict_wrapper(ZSTD_DCtx* ctx, uintptr_t dst, size_t dstCapacity, uintptr_t src, size_t srcSize, const ZSTD_DDict *ddict) {
    return ZSTD_decompress_usingDDict(ctx, (void*)dst, dstCapacity, (const void*)src, srcSize, ddict);
}

static unsigned long long ZSTD_getFrameContentSize_wrapper(uintptr_t src, size_t srcSize) {
    return ZSTD_getFrameContentSize((const void*)src, srcSize);
}
*/
import "C"

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"unsafe"
)

// DefaultCompressionLevel is the default compression level.
const DefaultCompressionLevel = 3 // Obtained from ZSTD_CLEVEL_DEFAULT.

// Compress appends compressed src to dst and returns the result.
func Compress(dst, src []byte) []byte {
	return compressDictLevel(dst, src, nil, DefaultCompressionLevel)
}

// CompressLevel appends compressed src to dst and returns the result.
//
// The given compressionLevel is used for the compression.
func CompressLevel(dst, src []byte, compressionLevel int) []byte {
	return compressDictLevel(dst, src, nil, compressionLevel)
}

// CompressDict appends compressed src to dst and returns the result.
//
// The given dictionary is used for the compression.
func CompressDict(dst, src []byte, cd *CDict) []byte {
	return compressDictLevel(dst, src, cd, 0)
}

func compressDictLevel(dst, src []byte, cd *CDict, compressionLevel int) []byte {
	concurrencyLimitCh <- struct{}{}

	var cctx, cctxDict *cctxWrapper
	if cd == nil {
		cctx = cctxPool.Get().(*cctxWrapper)
	} else {
		cctxDict = cctxDictPool.Get().(*cctxWrapper)
	}

	dst = compress(cctx, cctxDict, dst, src, cd, compressionLevel)

	if cd == nil {
		cctxPool.Put(cctx)
	} else {
		cctxDictPool.Put(cctxDict)
	}

	<-concurrencyLimitCh

	return dst
}

var cctxPool = &sync.Pool{
	New: newCCtx,
}

var cctxDictPool = &sync.Pool{
	New: newCCtx,
}

func newCCtx() interface{} {
	cctx := C.ZSTD_createCCtx()
	cw := &cctxWrapper{
		cctx: cctx,
	}
	runtime.SetFinalizer(cw, freeCCtx)
	return cw
}

func freeCCtx(cw *cctxWrapper) {
	C.ZSTD_freeCCtx(cw.cctx)
	cw.cctx = nil
}

type cctxWrapper struct {
	cctx *C.ZSTD_CCtx
}

func compress(cctx, cctxDict *cctxWrapper, dst, src []byte, cd *CDict, compressionLevel int) []byte {
	if len(src) == 0 {
		return dst
	}

	dstLen := len(dst)
	if cap(dst) > dstLen {
		// Fast path - try compressing without dst resize.
		result := compressInternal(cctx, cctxDict, dst[dstLen:cap(dst)], src, cd, compressionLevel, false)
		compressedSize := int(result)
		if compressedSize >= 0 {
			// All OK.
			return dst[:dstLen+compressedSize]
		}

		if C.ZSTD_getErrorCode(result) != C.ZSTD_error_dstSize_tooSmall {
			// Unexpected error.
			panic(fmt.Errorf("BUG: unexpected error during compression with cd=%p: %s", cd, errStr(result)))
		}
	}

	// Slow path - resize dst to fit compressed data.
	compressBound := int(C.ZSTD_compressBound(C.size_t(len(src)))) + 1
	if n := dstLen + compressBound - cap(dst) + dstLen; n > 0 {
		// This should be optimized since go 1.11 - see https://golang.org/doc/go1.11#performance-compiler.
		dst = append(dst[:cap(dst)], make([]byte, n)...)
	}

	result := compressInternal(cctx, cctxDict, dst[dstLen:dstLen+compressBound], src, cd, compressionLevel, true)
	compressedSize := int(result)
	return dst[:dstLen+compressedSize]
}

func compressInternal(cctx, cctxDict *cctxWrapper, dst, src []byte, cd *CDict, compressionLevel int, mustSucceed bool) C.size_t {
	if cd != nil {
		result := C.ZSTD_compress_usingCDict_wrapper(cctxDict.cctx,
			C.uintptr_t(uintptr(unsafe.Pointer(&dst[0]))),
			C.size_t(cap(dst)),
			C.uintptr_t(uintptr(unsafe.Pointer(&src[0]))),
			C.size_t(len(src)),
			cd.p)
		// Prevent from GC'ing of dst and src during CGO call above.
		runtime.KeepAlive(dst)
		runtime.KeepAlive(src)
		if mustSucceed {
			ensureNoError("ZSTD_compress_usingCDict_wrapper", result)
		}
		return result
	}
	result := C.ZSTD_compressCCtx_wrapper(cctx.cctx,
		C.uintptr_t(uintptr(unsafe.Pointer(&dst[0]))),
		C.size_t(cap(dst)),
		C.uintptr_t(uintptr(unsafe.Pointer(&src[0]))),
		C.size_t(len(src)),
		C.int(compressionLevel))
	// Prevent from GC'ing of dst and src during CGO call above.
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)
	if mustSucceed {
		ensureNoError("ZSTD_compressCCtx_wrapper", result)
	}
	return result
}

// Decompress appends decompressed src to dst and returns the result.
func Decompress(dst, src []byte) ([]byte, error) {
	return DecompressDict(dst, src, nil)
}

// DecompressDict appends decompressed src to dst and returns the result.
//
// The given dictionary dd is used for the decompression.
func DecompressDict(dst, src []byte, dd *DDict) ([]byte, error) {
	concurrencyLimitCh <- struct{}{}

	var dctx, dctxDict *dctxWrapper
	if dd == nil {
		dctx = dctxPool.Get().(*dctxWrapper)
	} else {
		dctxDict = dctxDictPool.Get().(*dctxWrapper)
	}

	var err error
	dst, err = decompress(dctx, dctxDict, dst, src, dd)

	if dd == nil {
		dctxPool.Put(dctx)
	} else {
		dctxDictPool.Put(dctxDict)
	}

	<-concurrencyLimitCh

	return dst, err
}

var dctxPool = &sync.Pool{
	New: newDCtx,
}

var dctxDictPool = &sync.Pool{
	New: newDCtx,
}

func newDCtx() interface{} {
	dctx := C.ZSTD_createDCtx()
	dw := &dctxWrapper{
		dctx: dctx,
	}
	runtime.SetFinalizer(dw, freeDCtx)
	return dw
}

func freeDCtx(dw *dctxWrapper) {
	C.ZSTD_freeDCtx(dw.dctx)
	dw.dctx = nil
}

type dctxWrapper struct {
	dctx *C.ZSTD_DCtx
}

func decompress(dctx, dctxDict *dctxWrapper, dst, src []byte, dd *DDict) ([]byte, error) {
	if len(src) == 0 {
		return dst, nil
	}

	dstLen := len(dst)
	if cap(dst) > dstLen {
		// Fast path - try decompressing without dst resize.
		result := decompressInternal(dctx, dctxDict, dst[dstLen:cap(dst)], src, dd)
		decompressedSize := int(result)
		if decompressedSize >= 0 {
			// All OK.
			return dst[:dstLen+decompressedSize], nil
		}

		if C.ZSTD_getErrorCode(result) != C.ZSTD_error_dstSize_tooSmall {
			// Error during decompression.
			return dst[:dstLen], fmt.Errorf("decompression error: %s", errStr(result))
		}
	}

	// Slow path - resize dst to fit decompressed data.
	decompressBound := int(C.ZSTD_getFrameContentSize_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(&src[0]))), C.size_t(len(src))))
	// Prevent from GC'ing of src during CGO call above.
	runtime.KeepAlive(src)
	switch uint64(decompressBound) {
	case uint64(C.ZSTD_CONTENTSIZE_UNKNOWN):
		return streamDecompress(dst, src, dd)
	case uint64(C.ZSTD_CONTENTSIZE_ERROR):
		return dst, fmt.Errorf("cannod decompress invalid src")
	}
	decompressBound++

	if n := dstLen + decompressBound - cap(dst); n > 0 {
		// This should be optimized since go 1.11 - see https://golang.org/doc/go1.11#performance-compiler.
		dst = append(dst[:cap(dst)], make([]byte, n)...)
	}

	result := decompressInternal(dctx, dctxDict, dst[dstLen:dstLen+decompressBound], src, dd)
	decompressedSize := int(result)
	if decompressedSize >= 0 {
		// All OK.
		return dst[:dstLen+decompressedSize], nil
	}

	// Error during decompression.
	return dst[:dstLen], fmt.Errorf("decompression error: %s", errStr(result))
}

func decompressInternal(dctx, dctxDict *dctxWrapper, dst, src []byte, dd *DDict) C.size_t {
	var n C.size_t
	if dd != nil {
		n = C.ZSTD_decompress_usingDDict_wrapper(dctxDict.dctx,
			C.uintptr_t(uintptr(unsafe.Pointer(&dst[0]))),
			C.size_t(cap(dst)),
			C.uintptr_t(uintptr(unsafe.Pointer(&src[0]))),
			C.size_t(len(src)),
			dd.p)
	} else {
		n = C.ZSTD_decompressDCtx_wrapper(dctx.dctx,
			C.uintptr_t(uintptr(unsafe.Pointer(&dst[0]))),
			C.size_t(cap(dst)),
			C.uintptr_t(uintptr(unsafe.Pointer(&src[0]))),
			C.size_t(len(src)))
	}
	// Prevent from GC'ing of dst and src during CGO calls above.
	runtime.KeepAlive(dst)
	runtime.KeepAlive(src)
	return n
}

var concurrencyLimitCh = func() chan struct{} {
	gomaxprocs := runtime.GOMAXPROCS(-1)
	return make(chan struct{}, gomaxprocs)
}()

func errStr(result C.size_t) string {
	errCode := C.ZSTD_getErrorCode(result)
	errCStr := C.ZSTD_getErrorString(errCode)
	return C.GoString(errCStr)
}

func ensureNoError(funcName string, result C.size_t) {
	if int(result) >= 0 {
		// Fast path - avoid calling C function.
		return
	}
	if C.ZSTD_getErrorCode(result) != 0 {
		panic(fmt.Errorf("BUG: unexpected error in %s: %s", funcName, errStr(result)))
	}
}

func streamDecompress(dst, src []byte, dd *DDict) ([]byte, error) {
	sd := getStreamDecompressor(dd)
	sd.dst = dst
	sd.src = src
	_, err := sd.zr.WriteTo(sd)
	dst = sd.dst
	putStreamDecompressor(sd)
	return dst, err
}

type streamDecompressor struct {
	dst       []byte
	src       []byte
	srcOffset int

	zr *Reader
}

type srcReader streamDecompressor

func (sr *srcReader) Read(p []byte) (int, error) {
	sd := (*streamDecompressor)(sr)
	n := copy(p, sd.src[sd.srcOffset:])
	sd.srcOffset += n
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (sd *streamDecompressor) Write(p []byte) (int, error) {
	sd.dst = append(sd.dst, p...)
	return len(p), nil
}

func getStreamDecompressor(dd *DDict) *streamDecompressor {
	v := streamDecompressorPool.Get()
	if v == nil {
		sd := &streamDecompressor{
			zr: NewReader(nil),
		}
		v = sd
	}
	sd := v.(*streamDecompressor)
	sd.zr.Reset((*srcReader)(sd), dd)
	return sd
}

func putStreamDecompressor(sd *streamDecompressor) {
	sd.dst = nil
	sd.src = nil
	sd.srcOffset = 0
	sd.zr.Reset(nil, nil)
	streamDecompressorPool.Put(sd)
}

var streamDecompressorPool sync.Pool
