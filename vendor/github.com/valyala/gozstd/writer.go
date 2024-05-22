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

static size_t ZSTD_CCtx_setParameter_wrapper(uintptr_t cs, ZSTD_cParameter param, int value) {
    return ZSTD_CCtx_setParameter((ZSTD_CStream*)cs, param, value);
}

static size_t ZSTD_initCStream_wrapper(uintptr_t cs, int compressionLevel) {
    return ZSTD_initCStream((ZSTD_CStream*)cs, compressionLevel);
}

static size_t ZSTD_CCtx_refCDict_wrapper(uintptr_t cc, uintptr_t dict) {
    return ZSTD_CCtx_refCDict((ZSTD_CCtx*)cc, (ZSTD_CDict*)dict);
}

static size_t ZSTD_freeCStream_wrapper(uintptr_t cs) {
    return ZSTD_freeCStream((ZSTD_CStream*)cs);
}

static size_t ZSTD_compressStream_wrapper(uintptr_t cs, uintptr_t output, uintptr_t input) {
    return ZSTD_compressStream((ZSTD_CStream*)cs, (ZSTD_outBuffer*)output, (ZSTD_inBuffer*)input);
}

static size_t ZSTD_flushStream_wrapper(uintptr_t cs, uintptr_t output) {
    return ZSTD_flushStream((ZSTD_CStream*)cs, (ZSTD_outBuffer*)output);
}

static size_t ZSTD_endStream_wrapper(uintptr_t cs, uintptr_t output) {
    return ZSTD_endStream((ZSTD_CStream*)cs, (ZSTD_outBuffer*)output);
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
	cstreamInBufSize  = C.ZSTD_CStreamInSize()
	cstreamOutBufSize = C.ZSTD_CStreamOutSize()
)

type cMemPtr *[1 << 30]byte

// Writer implements zstd writer.
type Writer struct {
	w                io.Writer
	compressionLevel int
	wlog             int
	cs               *C.ZSTD_CStream
	cd               *CDict

	inBuf  *C.ZSTD_inBuffer
	outBuf *C.ZSTD_outBuffer

	inBufGo  cMemPtr
	outBufGo cMemPtr
}

// NewWriter returns new zstd writer writing compressed data to w.
//
// The returned writer must be closed with Close call in order
// to finalize the compressed stream.
//
// Call Release when the Writer is no longer needed.
func NewWriter(w io.Writer) *Writer {
	return NewWriterParams(w, nil)
}

// NewWriterLevel returns new zstd writer writing compressed data to w
// at the given compression level.
//
// The returned writer must be closed with Close call in order
// to finalize the compressed stream.
//
// Call Release when the Writer is no longer needed.
func NewWriterLevel(w io.Writer, compressionLevel int) *Writer {
	params := &WriterParams{
		CompressionLevel: compressionLevel,
	}
	return NewWriterParams(w, params)
}

// NewWriterDict returns new zstd writer writing compressed data to w
// using the given cd.
//
// The returned writer must be closed with Close call in order
// to finalize the compressed stream.
//
// Call Release when the Writer is no longer needed.
func NewWriterDict(w io.Writer, cd *CDict) *Writer {
	params := &WriterParams{
		Dict: cd,
	}
	return NewWriterParams(w, params)
}

const (
	// WindowLogMin is the minimum value of the windowLog parameter.
	WindowLogMin = 10 // from zstd.h
	// WindowLogMax32 is the maximum value of the windowLog parameter on 32-bit architectures.
	WindowLogMax32 = 30 // from zstd.h
	// WindowLogMax64 is the maximum value of the windowLog parameter on 64-bit architectures.
	WindowLogMax64 = 31 // from zstd.h

	// DefaultWindowLog is the default value of the windowLog parameter.
	DefaultWindowLog = 0
)

// A WriterParams allows users to specify compression parameters by calling
// NewWriterParams.
//
// Calling NewWriterParams with a nil WriterParams is equivalent to calling
// NewWriter.
type WriterParams struct {
	// Compression level. Special value 0 means 'default compression level'.
	CompressionLevel int

	// WindowLog. Must be clamped between WindowLogMin and WindowLogMin32/64.
	// Special value 0 means 'use default windowLog'.
	//
	// Note: enabling log distance matching increases memory usage for both
	// compressor and decompressor. When set to a value greater than 27, the
	// decompressor requires special treatment.
	WindowLog int

	// Dict is optional dictionary used for compression.
	Dict *CDict
}

// NewWriterParams returns new zstd writer writing compressed data to w
// using the given set of parameters.
//
// The returned writer must be closed with Close call in order
// to finalize the compressed stream.
//
// Call Release when the Writer is no longer needed.
func NewWriterParams(w io.Writer, params *WriterParams) *Writer {
	if params == nil {
		params = &WriterParams{}
	}

	cs := C.ZSTD_createCStream()
	initCStream(cs, *params)

	inBuf := (*C.ZSTD_inBuffer)(C.calloc(1, C.sizeof_ZSTD_inBuffer))
	inBuf.src = C.calloc(1, cstreamInBufSize)
	inBuf.size = 0
	inBuf.pos = 0

	outBuf := (*C.ZSTD_outBuffer)(C.calloc(1, C.sizeof_ZSTD_outBuffer))
	outBuf.dst = C.calloc(1, cstreamOutBufSize)
	outBuf.size = cstreamOutBufSize
	outBuf.pos = 0

	zw := &Writer{
		w:                w,
		compressionLevel: params.CompressionLevel,
		wlog:             params.WindowLog,
		cs:               cs,
		cd:               params.Dict,
		inBuf:            inBuf,
		outBuf:           outBuf,
	}

	zw.inBufGo = cMemPtr(zw.inBuf.src)
	zw.outBufGo = cMemPtr(zw.outBuf.dst)

	runtime.SetFinalizer(zw, freeCStream)
	return zw
}

// Reset resets zw to write to w using the given dictionary cd and the given
// compressionLevel. Use ResetWriterParams if you wish to change other
// parameters that were set via WriterParams.
func (zw *Writer) Reset(w io.Writer, cd *CDict, compressionLevel int) {
	params := WriterParams{
		CompressionLevel: compressionLevel,
		WindowLog:        zw.wlog,
		Dict:             cd,
	}
	zw.ResetWriterParams(w, &params)
}

// ResetWriterParams resets zw to write to w using the given set of parameters.
func (zw *Writer) ResetWriterParams(w io.Writer, params *WriterParams) {
	zw.inBuf.size = 0
	zw.inBuf.pos = 0
	zw.outBuf.size = cstreamOutBufSize
	zw.outBuf.pos = 0

	zw.cd = params.Dict
	initCStream(zw.cs, *params)

	zw.w = w
}

func initCStream(cs *C.ZSTD_CStream, params WriterParams) {
	if params.Dict != nil {
		result := C.ZSTD_CCtx_refCDict_wrapper(
			C.uintptr_t(uintptr(unsafe.Pointer(cs))),
			C.uintptr_t(uintptr(unsafe.Pointer(params.Dict.p))))
		ensureNoError("ZSTD_CCtx_refCDict", result)
	} else {
		result := C.ZSTD_initCStream_wrapper(
			C.uintptr_t(uintptr(unsafe.Pointer(cs))),
			C.int(params.CompressionLevel))
		ensureNoError("ZSTD_initCStream", result)
	}

	result := C.ZSTD_CCtx_setParameter_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(cs))),
		C.ZSTD_cParameter(C.ZSTD_c_windowLog),
		C.int(params.WindowLog))
	ensureNoError("ZSTD_CCtx_setParameter", result)
}

func freeCStream(v interface{}) {
	v.(*Writer).Release()
}

// Release releases all the resources occupied by zw.
//
// zw cannot be used after the release.
func (zw *Writer) Release() {
	if zw.cs == nil {
		return
	}

	result := C.ZSTD_freeCStream_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(zw.cs))))
	ensureNoError("ZSTD_freeCStream", result)
	zw.cs = nil

	C.free(unsafe.Pointer(zw.inBuf.src))
	C.free(unsafe.Pointer(zw.inBuf))
	zw.inBuf = nil

	C.free(unsafe.Pointer(zw.outBuf.dst))
	C.free(unsafe.Pointer(zw.outBuf))
	zw.outBuf = nil

	zw.w = nil
	zw.cd = nil
}

// ReadFrom reads all the data from r and writes it to zw.
//
// Returns the number of bytes read from r.
//
// ReadFrom may not flush the compressed data to the underlying writer
// due to performance reasons.
// Call Flush or Close when the compressed data must propagate
// to the underlying writer.
func (zw *Writer) ReadFrom(r io.Reader) (int64, error) {
	nn := int64(0)
	for {
		// Fill the inBuf.
		for zw.inBuf.size < cstreamInBufSize {
			n, err := r.Read(zw.inBufGo[zw.inBuf.size:cstreamInBufSize])

			// Sometimes n > 0 even when Read() returns an error.
			// This is true especially if the error is io.EOF.
			zw.inBuf.size += C.size_t(n)
			nn += int64(n)

			if err != nil {
				if err == io.EOF {
					return nn, nil
				}
				return nn, err
			}
		}

		// Flush the inBuf.
		if err := zw.flushInBuf(); err != nil {
			return nn, err
		}
	}
}

// Write writes p to zw.
//
// Write doesn't flush the compressed data to the underlying writer
// due to performance reasons.
// Call Flush or Close when the compressed data must propagate
// to the underlying writer.
func (zw *Writer) Write(p []byte) (int, error) {
	pLen := len(p)
	if pLen == 0 {
		return 0, nil
	}

	for {
		n := copy(zw.inBufGo[zw.inBuf.size:cstreamInBufSize], p)
		zw.inBuf.size += C.size_t(n)
		p = p[n:]
		if len(p) == 0 {
			// Fast path - just copy the data to input buffer.
			return pLen, nil
		}
		if err := zw.flushInBuf(); err != nil {
			return 0, err
		}
	}
}

func (zw *Writer) flushInBuf() error {
	prevInBufPos := zw.inBuf.pos
	result := C.ZSTD_compressStream_wrapper(
		C.uintptr_t(uintptr(unsafe.Pointer(zw.cs))),
		C.uintptr_t(uintptr(unsafe.Pointer(zw.outBuf))),
		C.uintptr_t(uintptr(unsafe.Pointer(zw.inBuf))))
	ensureNoError("ZSTD_compressStream", result)

	// Move the remaining data to the start of inBuf.
	copy(zw.inBufGo[:cstreamInBufSize], zw.inBufGo[zw.inBuf.pos:zw.inBuf.size])
	zw.inBuf.size -= zw.inBuf.pos
	zw.inBuf.pos = 0

	if zw.outBuf.size-zw.outBuf.pos > zw.outBuf.pos && prevInBufPos != zw.inBuf.pos {
		// There is enough space in outBuf and the last compression
		// succeeded, so don't flush outBuf yet.
		return nil
	}

	// Flush outBuf, since there is low space in it or the last compression
	// attempt was unsuccessful.
	return zw.flushOutBuf()
}

func (zw *Writer) flushOutBuf() error {
	if zw.outBuf.pos == 0 {
		// Nothing to flush.
		return nil
	}

	outBuf := zw.outBufGo[:zw.outBuf.pos]
	n, err := zw.w.Write(outBuf)
	zw.outBuf.pos = 0
	if err != nil {
		return fmt.Errorf("cannot flush internal buffer to the underlying writer: %s", err)
	}
	if n != len(outBuf) {
		panic(fmt.Errorf("BUG: the underlying writer violated io.Writer contract and didn't return error after writing incomplete data; written %d bytes; want %d bytes",
			n, len(outBuf)))
	}
	return nil
}

// Flush flushes the remaining data from zw to the underlying writer.
func (zw *Writer) Flush() error {
	// Flush inBuf.
	for zw.inBuf.size > 0 {
		if err := zw.flushInBuf(); err != nil {
			return err
		}
	}

	// Flush the internal buffer to outBuf.
	for {
		result := C.ZSTD_flushStream_wrapper(
			C.uintptr_t(uintptr(unsafe.Pointer(zw.cs))),
			C.uintptr_t(uintptr(unsafe.Pointer(zw.outBuf))))
		ensureNoError("ZSTD_flushStream", result)
		if err := zw.flushOutBuf(); err != nil {
			return err
		}
		if result == 0 {
			// No more data left in the internal buffer.
			return nil
		}
	}
}

// Close finalizes the compressed stream and flushes all the compressed data
// to the underlying writer.
//
// It doesn't close the underlying writer passed to New* functions.
func (zw *Writer) Close() error {
	if err := zw.Flush(); err != nil {
		return err
	}

	for {
		result := C.ZSTD_endStream_wrapper(
			C.uintptr_t(uintptr(unsafe.Pointer(zw.cs))),
			C.uintptr_t(uintptr(unsafe.Pointer(zw.outBuf))))
		ensureNoError("ZSTD_endStream", result)
		if err := zw.flushOutBuf(); err != nil {
			return err
		}
		if result == 0 {
			return nil
		}
	}
}
