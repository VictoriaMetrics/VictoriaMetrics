package protoparserutil

import (
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/snappy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ioutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

// snappy has default limit of 2_704_094_487 ( 2 GB)
// which is too high for common VictoriaMetrics insert requests
// limit to 56MB in order to prevent possible memory allocation attacks
//
// Later we could consider to make this limit configurable
const maxSnappyBlockSize = 56_000_000

// ReadUncompressedData reads uncompressed data from r using the given encoding and then passes it to the callback.
//
// The maxDataSize limits the maximum data size, which can be read from r.
//
// The callback must not hold references to the data after returning.
func ReadUncompressedData(r io.Reader, contentType string, maxDataSize *flagutil.Bytes, callback func(data []byte) error) error {
	fbr := ioutil.GetFirstByteReader(r)
	defer ioutil.PutFirstByteReader(fbr)

	// Wait for the first byte before obtaining the concurrency token
	// and allocating resources needed for reading and processing the data from r.
	// This should prevent from allocating concurrency tokens and memory
	// for connections without incoming data.
	fbr.WaitForData()

	if err := writeconcurrencylimiter.IncConcurrency(); err != nil {
		return err
	}
	defer writeconcurrencylimiter.DecConcurrency()

	if contentType == "zstd" {
		// Fast path for zstd contentType - read the data in full and then decompress it by a single call.
		dcompress := func(dst, src []byte) ([]byte, error) {
			return encoding.DecompressZSTDLimited(dst, src, maxDataSize.IntN())
		}
		return readUncompressedData(fbr, maxDataSize, dcompress, callback)
	}
	if contentType == "snappy" {
		// Special case for snappy. The snappy data must be read in full and then decompressed,
		// since streaming snappy encoding is incompatible with block snappy encoding.
		decompress := func(dst, src []byte) ([]byte, error) {
			return snappy.Decode(dst, src, maxDataSize.IntN())
		}
		return readUncompressedData(fbr, maxDataSize, decompress, callback)
	}

	// Slow path for other supported protocol encoders.
	reader, err := GetUncompressedReader(fbr, contentType)
	if err != nil {
		return err
	}
	defer PutUncompressedReader(reader)

	return readFull(reader, maxDataSize, callback)
}

func readUncompressedData(r io.Reader, maxDataSize *flagutil.Bytes, decompress func(dst, src []byte) ([]byte, error), callback func(data []byte) error) error {
	return readFull(r, maxDataSize, func(data []byte) error {
		dbb := decompressedBufPool.Get()
		defer decompressedBufPool.Put(dbb)

		var err error
		dbb.B, err = decompress(dbb.B, data)
		if err != nil {
			return fmt.Errorf("cannot decompress data: %w", err)
		}
		if int64(len(dbb.B)) > maxDataSize.N {
			return fmt.Errorf("too big decompressed data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
		}

		return callback(dbb.B)
	})
}

func readFull(r io.Reader, maxDataSize *flagutil.Bytes, callback func(data []byte) error) error {
	lr := ioutil.GetLimitedReader(r, maxDataSize.N+1)
	defer ioutil.PutLimitedReader(lr)

	bb := fullReaderBufPool.Get()
	defer func() {
		if len(bb.B) > 1024*1024 && cap(bb.B) > 4*len(bb.B) {
			// Do not store too big bb to the pool if only a small part of the buffer is used last time.
			// This should reduce memory waste.
			return
		}
		fullReaderBufPool.Put(bb)
	}()

	if _, err := bb.ReadFrom(lr); err != nil {
		return err
	}

	if int64(len(bb.B)) > maxDataSize.N {
		return fmt.Errorf("too big data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
	}

	return callback(bb.B)
}

var fullReaderBufPool bytesutil.ByteBufferPool

var (
	compressedBufPool   bytesutil.ByteBufferPool
	decompressedBufPool bytesutil.ByteBufferPool
)

// GetUncompressedReader returns uncompressed reader for r and the given contentType
//
// The returned reader must be passed to PutUncompressedReader when no longer needed.
func GetUncompressedReader(r io.Reader, contentType string) (io.Reader, error) {
	switch contentType {
	case "zstd":
		return zstd.GetReader(r), nil
	case "snappy":
		return getSnappyReader(r)
	case "gzip":
		return getGzipReader(r)
	case "deflate":
		return getZlibReader(r)
	case "", "none", "identity":
		// Datadog extensions sends Content-Encoding: identity, which is not supported by RFC 2616
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8649
		return getPlainReader(r), nil
	default:
		return nil, fmt.Errorf("unsupported contentType: %s", contentType)
	}
}

// PutUncompressedReader puts r to the pool, so it could be reused via GetUncompressedReader()
func PutUncompressedReader(r io.Reader) {
	switch t := r.(type) {
	case *snappyReader:
		putSnappyReader(t)
	case *zstd.Reader:
		zstd.PutReader(t)
	case *gzip.Reader:
		putGzipReader(t)
	case zlib.Resetter:
		putZlibReader(t)
	case *plainReader:
		putPlainReader(t)
	default:
		logger.Panicf("BUG: unsupported reader passed to PutUncompressedReader: %T", r)
	}
}

type plainReader struct {
	r io.Reader
}

func (pr *plainReader) Read(p []byte) (int, error) {
	return pr.r.Read(p)
}

func getPlainReader(r io.Reader) *plainReader {
	v := plainReaderPool.Get()
	if v == nil {
		v = &plainReader{}
	}
	pr := v.(*plainReader)
	pr.r = r
	return pr
}

func putPlainReader(pr *plainReader) {
	pr.r = nil
	plainReaderPool.Put(pr)
}

var plainReaderPool sync.Pool

func getGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func putGzipReader(zr *gzip.Reader) {
	gzipReaderPool.Put(zr)
}

var gzipReaderPool sync.Pool

func getZlibReader(r io.Reader) (io.ReadCloser, error) {
	v := zlibReaderPool.Get()
	if v == nil {
		return zlib.NewReader(r)
	}
	zr := v.(zlib.Resetter)
	if err := zr.Reset(r, nil); err != nil {
		return nil, err
	}
	return zr.(io.ReadCloser), nil
}

func putZlibReader(zr zlib.Resetter) {
	zlibReaderPool.Put(zr)
}

var zlibReaderPool sync.Pool

type snappyReader struct {
	// b contains decompressed the data, which must be read by snappy reader
	b []byte

	// offset is an offset at b for the remaining data to read
	offset int
}

func (sr *snappyReader) Reset(r io.Reader) error {
	// Read the whole data in one go, since it is expected that Snappy data
	// is compressed in block mode instead of stream mode.
	// See https://pkg.go.dev/github.com/golang/snappy
	cbb := compressedBufPool.Get()
	_, err := cbb.ReadFrom(r)
	if err != nil {
		compressedBufPool.Put(cbb)
		return fmt.Errorf("cannot read snappy-encoded data block: %w", err)
	}
	sr.b, err = snappy.Decode(sr.b, cbb.B, maxSnappyBlockSize)
	compressedBufPool.Put(cbb)
	sr.offset = 0
	if err != nil {
		return fmt.Errorf("cannot decode snappy-encoded data block: %w", err)
	}
	return err
}

func (sr *snappyReader) Read(p []byte) (int, error) {
	if sr.offset >= len(sr.b) {
		return 0, io.EOF
	}
	n := copy(p, sr.b[sr.offset:])
	sr.offset += n
	if n == len(p) {
		return n, nil
	}
	return n, io.EOF
}

func getSnappyReader(r io.Reader) (*snappyReader, error) {
	v := snappyReaderPool.Get()
	if v == nil {
		v = &snappyReader{}
	}
	zr := v.(*snappyReader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func putSnappyReader(zr *snappyReader) {
	snappyReaderPool.Put(zr)
}

var snappyReaderPool sync.Pool
