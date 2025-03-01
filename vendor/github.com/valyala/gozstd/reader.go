package gozstd

/*
#cgo CFLAGS: -O3

#define ZSTD_STATIC_LINKING_ONLY
#include "zstd.h"
#include "zstd_errors.h"

#include <stdlib.h>  // for malloc/free
#include <stdint.h>  // for uintptr_t

// The following *_wrapper functions allow avoiding memory allocations
// durting calls from Go.
// See https://github.com/golang/go/issues/24450 .

static size_t ZSTD_initDCtx_usingDDict_wrapper(uintptr_t dctx, uintptr_t ddict) {
    ZSTD_DCtx *zdctx = (ZSTD_DCtx *)dctx;
    size_t rv = ZSTD_DCtx_reset(zdctx, ZSTD_reset_session_only);
    if (rv != 0) {
        return rv;
    }
    return ZSTD_DCtx_refDDict(zdctx, (ZSTD_DDict *)ddict);
}

static size_t ZSTD_freeDCtx_wrapper(uintptr_t dctx) {
    return ZSTD_freeDCtx((ZSTD_DCtx*)dctx);
}

static size_t ZSTD_decompressStream_wrapper(uintptr_t dctx, uintptr_t output, uintptr_t input) {
    return ZSTD_decompressStream((ZSTD_DCtx*)dctx, (ZSTD_outBuffer*)output, (ZSTD_inBuffer*)input);
}
*/
import "C"

import (
	"fmt"
	"io"
	"runtime"
	"unsafe"
)

var (
	dstreamInBufSize  = C.ZSTD_DStreamInSize()
	dstreamOutBufSize = C.ZSTD_DStreamOutSize()
)

// Reader implements zstd reader.
type Reader struct {
	r    io.Reader
	dctx *C.ZSTD_DCtx
	dd   *DDict

	inBuf  *C.ZSTD_inBuffer
	outBuf *C.ZSTD_outBuffer

	inBufGo  cMemPtr
	outBufGo cMemPtr
}

// MustNewReader returns new zstd reader reading compressed data from r and panics in case of error.
//
// Call Close when the Reader is no longer needed.
func MustNewReader(r io.Reader) *Reader {
	return MustNewReaderDict(r, nil)
}

// NewReader returns new zstd reader reading compressed data from r.
//
// Call Close when the Reader is no longer needed.
func NewReader(r io.Reader) (*Reader, error) {
	return NewReaderDict(r, nil)
}

// MustNewReaderDict returns new zstd reader reading compressed data from r and panics in case of error.
func MustNewReaderDict(r io.Reader, dd *DDict) *Reader {
	zr, err := NewReaderDict(r, dd)
	if err != nil {
		panic(err)
	}
	return zr
}

// NewReaderDict returns new zstd reader reading compressed data from r
// using the given DDict.
//
// Call Close when the Reader is no longer needed.
func NewReaderDict(r io.Reader, dd *DDict) (*Reader, error) {
	dctx := C.ZSTD_createDCtx()
	err := initDCtx(dctx, dd)
	if err != nil {
		return nil, err
	}

	zr := &Reader{
		r:    r,
		dctx: dctx,
		dd:   dd,
	}

	zr.inBuf = (*C.ZSTD_inBuffer)(C.calloc(1, C.sizeof_ZSTD_inBuffer))
	zr.inBuf.src = C.calloc(1, dstreamInBufSize)
	zr.inBuf.size = 0
	zr.inBuf.pos = 0

	zr.outBuf = (*C.ZSTD_outBuffer)(C.calloc(1, C.sizeof_ZSTD_outBuffer))
	zr.outBuf.dst = C.calloc(1, dstreamOutBufSize)
	zr.outBuf.size = 0
	zr.outBuf.pos = 0

	zr.inBufGo = cMemPtr(zr.inBuf.src)
	zr.outBufGo = cMemPtr(zr.outBuf.dst)

	runtime.SetFinalizer(zr, freeReader)
	return zr, nil
}

// Reset resets zr to read from r.
func (zr *Reader) Reset(r io.Reader) error {
	return zr.ResetWithDict(r, nil)
}

// MustReset resets zr to read from r using the given dictionary dd and panics in case of error.
func (zr *Reader) MustReset(r io.Reader, dd *DDict) {
	if err := zr.ResetWithDict(r, dd); err != nil {
		panic(err)
	}
}

// ResetWithDict resets zr to read from r using the given dictionary dd.
func (zr *Reader) ResetWithDict(r io.Reader, dd *DDict) error {
	if zr.inBuf == nil {
		zr.inBuf = (*C.ZSTD_inBuffer)(C.calloc(1, C.sizeof_ZSTD_inBuffer))
		zr.inBuf.src = C.calloc(1, dstreamInBufSize)
		zr.inBufGo = cMemPtr(zr.inBuf.src)
	}
	zr.inBuf.size = 0
	zr.inBuf.pos = 0
	if zr.outBuf == nil {
		zr.outBuf = (*C.ZSTD_outBuffer)(C.calloc(1, C.sizeof_ZSTD_outBuffer))
		zr.outBuf.dst = C.calloc(1, dstreamOutBufSize)
		zr.outBufGo = cMemPtr(zr.outBuf.dst)
	}
	zr.outBuf.size = 0
	zr.outBuf.pos = 0

	if zr.dctx == nil {
		zr.dctx = C.ZSTD_createDCtx()
	}
	zr.dd = dd
	zr.r = r

	return initDCtx(zr.dctx, zr.dd)
}

func initDCtx(dctx *C.ZSTD_DCtx, dd *DDict) error {
	var ddict *C.ZSTD_DDict
	if dd != nil {
		ddict = dd.p
	}
	result := C.ZSTD_initDCtx_usingDDict_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(dctx))),
		C.uintptr_t(uintptr(unsafe.Pointer(ddict))))
	return getError("ZSTD_initDCtx_usingDDict", result)
}

func freeReader(v any) {
	v.(*Reader).Close()
}

// Close releases all the resources occupied by zr.
//
// zr cannot be used after the release.
func (zr *Reader) Close() {
	if zr.dctx == nil {
		return
	}

	result := C.ZSTD_freeDCtx_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(zr.dctx))))
	ensureNoError("ZSTD_freeDCtx", result)
	zr.dctx = nil

	C.free(zr.inBuf.src)
	C.free(unsafe.Pointer(zr.inBuf))
	zr.inBuf = nil

	C.free(zr.outBuf.dst)
	C.free(unsafe.Pointer(zr.outBuf))
	zr.outBuf = nil

	zr.r = nil
	zr.dd = nil
}

// WriteTo writes all the data from zr to w.
//
// It returns the number of bytes written to w.
func (zr *Reader) WriteTo(w io.Writer) (int64, error) {
	nn := int64(0)
	for {
		if zr.outBuf.pos == zr.outBuf.size {
			if err := zr.fillOutBuf(); err != nil {
				if err == io.EOF {
					return nn, nil
				}
				return nn, err
			}
		}
		n, err := w.Write(zr.outBufGo[zr.outBuf.pos:zr.outBuf.size])
		zr.outBuf.pos += C.size_t(n)
		nn += int64(n)
		if err != nil {
			return nn, err
		}
	}
}

// Read reads up to len(p) bytes from zr to p.
func (zr *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if zr.outBuf.pos == zr.outBuf.size {
		if err := zr.fillOutBuf(); err != nil {
			return 0, err
		}
	}

	n := copy(p, zr.outBufGo[zr.outBuf.pos:zr.outBuf.size])
	zr.outBuf.pos += C.size_t(n)
	return n, nil
}

func (zr *Reader) fillOutBuf() error {
	if zr.inBuf.pos == zr.inBuf.size && zr.outBuf.size < dstreamOutBufSize {
		// inBuf is empty and the previously decompressed data size
		// is smaller than the maximum possible zr.outBuf.size.
		// This means that the internal buffer in zr.dctx doesn't contain
		// more data to decompress, so read new data into inBuf.
		if err := zr.fillInBuf(); err != nil {
			return err
		}
	}

tryDecompressAgain:
	// Try decompressing inBuf into outBuf.
	zr.outBuf.size = dstreamOutBufSize
	zr.outBuf.pos = 0
	prevInBufPos := zr.inBuf.pos
	result := C.ZSTD_decompressStream_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(zr.dctx))),
		C.uintptr_t(uintptr(unsafe.Pointer(zr.outBuf))),
		C.uintptr_t(uintptr(unsafe.Pointer(zr.inBuf))))
	zr.outBuf.size = zr.outBuf.pos
	zr.outBuf.pos = 0

	if C.ZSTD_getErrorCode(result) != 0 {
		return fmt.Errorf("cannot decompress data: %s", errStr(result))
	}

	if zr.outBuf.size > 0 {
		// Something has been decompressed to outBuf. Return it.
		return nil
	}

	// Nothing has been decompressed from inBuf.
	if zr.inBuf.pos != prevInBufPos && zr.inBuf.pos < zr.inBuf.size {
		// Data has been consumed from inBuf, but decompressed
		// into nothing. There is more data in inBuf, so try
		// decompressing it again.
		goto tryDecompressAgain
	}

	// Either nothing has been consumed from inBuf or it has been
	// decompressed into nothing and inBuf became empty.
	// Read more data into inBuf and try decompressing again.
	if err := zr.fillInBuf(); err != nil {
		return err
	}
	goto tryDecompressAgain
}

func (zr *Reader) fillInBuf() error {
	// Copy the remaining data to the start of inBuf.
	copy(zr.inBufGo[:dstreamInBufSize], zr.inBufGo[zr.inBuf.pos:zr.inBuf.size])
	zr.inBuf.size -= zr.inBuf.pos
	zr.inBuf.pos = 0

readAgain:
	// Read more data into inBuf.
	n, err := zr.r.Read(zr.inBufGo[zr.inBuf.size:dstreamInBufSize])
	zr.inBuf.size += C.size_t(n)
	if err == nil {
		if n == 0 {
			// Nothing has been read. Try reading data again.
			goto readAgain
		}
		return nil
	}
	if n > 0 {
		// Do not return error if at least a single byte read, i.e. forward progress is made.
		return nil
	}
	if err == io.EOF {
		// Do not wrap io.EOF, so the caller may notify the end of stream.
		return err
	}
	return fmt.Errorf("cannot read data from the underlying reader: %s", err)
}
