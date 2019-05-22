package gozstd

/*
#define ZSTD_STATIC_LINKING_ONLY
#include "zstd.h"
#include "zstd_errors.h"

#include <stdlib.h>  // for malloc/free
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
	r  io.Reader
	ds *C.ZSTD_DStream
	dd *DDict

	inBuf  *C.ZSTD_inBuffer
	outBuf *C.ZSTD_outBuffer

	inBufGo  cMemPtr
	outBufGo cMemPtr
}

// NewReader returns new zstd reader reading compressed data from r.
//
// Call Release when the Reader is no longer needed.
func NewReader(r io.Reader) *Reader {
	return NewReaderDict(r, nil)
}

// NewReaderDict returns new zstd reader reading compressed data from r
// using the given DDict.
//
// Call Release when the Reader is no longer needed.
func NewReaderDict(r io.Reader, dd *DDict) *Reader {
	ds := C.ZSTD_createDStream()
	initDStream(ds, dd)

	inBuf := (*C.ZSTD_inBuffer)(C.malloc(C.sizeof_ZSTD_inBuffer))
	inBuf.src = C.malloc(dstreamInBufSize)
	inBuf.size = 0
	inBuf.pos = 0

	outBuf := (*C.ZSTD_outBuffer)(C.malloc(C.sizeof_ZSTD_outBuffer))
	outBuf.dst = C.malloc(dstreamOutBufSize)
	outBuf.size = 0
	outBuf.pos = 0

	zr := &Reader{
		r:      r,
		ds:     ds,
		dd:     dd,
		inBuf:  inBuf,
		outBuf: outBuf,
	}

	zr.inBufGo = cMemPtr(zr.inBuf.src)
	zr.outBufGo = cMemPtr(zr.outBuf.dst)

	runtime.SetFinalizer(zr, freeDStream)
	return zr
}

// Reset resets zr to read from r using the given dictionary dd.
func (zr *Reader) Reset(r io.Reader, dd *DDict) {
	zr.inBuf.size = 0
	zr.inBuf.pos = 0
	zr.outBuf.size = 0
	zr.outBuf.pos = 0

	zr.dd = dd
	initDStream(zr.ds, zr.dd)

	zr.r = r
}

func initDStream(ds *C.ZSTD_DStream, dd *DDict) {
	var ddict *C.ZSTD_DDict
	if dd != nil {
		ddict = dd.p
	}
	result := C.ZSTD_initDStream_usingDDict(ds, ddict)
	ensureNoError("ZSTD_initDStream_usingDDict", result)
}

func freeDStream(v interface{}) {
	v.(*Reader).Release()
}

// Release releases all the resources occupied by zr.
//
// zr cannot be used after the release.
func (zr *Reader) Release() {
	if zr.ds == nil {
		return
	}

	result := C.ZSTD_freeDStream(zr.ds)
	ensureNoError("ZSTD_freeDStream", result)
	zr.ds = nil

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
		// This means that the internal buffer in zr.ds doesn't contain
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
	result := C.ZSTD_decompressStream(zr.ds, zr.outBuf, zr.inBuf)
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
	if err != io.EOF {
		return fmt.Errorf("cannot read data from the underlying reader: %s", err)
	}
	if n == 0 {
		return io.EOF
	}
	return nil
}
