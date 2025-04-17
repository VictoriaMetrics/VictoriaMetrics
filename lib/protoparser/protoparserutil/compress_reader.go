package protoparserutil

import (
	"fmt"
	"io"
	"sync"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

// ReadUncompressedData reads uncompressed data from r using the given encoding and then passes it to the callback.
//
// The maxDataSize limits the maximum data size, which can be read from r.
//
// The callback must not hold references to the data after returning.
func ReadUncompressedData(r io.Reader, encoding string, maxDataSize *flagutil.Bytes, callback func(data []byte) error) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	if encoding == "zstd" {
		// Fast path for zstd encoding - read the data in full and then decompress it by a single call.
		return readUncompressedData(wcr, maxDataSize, zstd.Decompress, callback)
	}
	if encoding == "snappy" {
		// Special case for snappy. The snappy data must be read in full and then decompressed,
		// since streaming snappy encoding is incompatible with block snappy encoding.
		decompress := func(dst, src []byte) ([]byte, error) {
			return snappy.Decode(dst[:cap(dst)], src)
		}
		return readUncompressedData(wcr, maxDataSize, decompress, callback)
	}

	// Slow path for other supported protocol encoders.
	reader, err := GetUncompressedReader(wcr, encoding)
	if err != nil {
		return err
	}
	lr := io.LimitReader(reader, maxDataSize.N+1)

	dbb := decompressedBufPool.Get()
	defer decompressedBufPool.Put(dbb)

	_, err = dbb.ReadFrom(lr)
	PutUncompressedReader(reader)
	if err != nil {
		return err
	}
	if int64(len(dbb.B)) > maxDataSize.N {
		return fmt.Errorf("too big data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
	}

	return callback(dbb.B)
}

func readUncompressedData(r io.Reader, maxDataSize *flagutil.Bytes, decompress func(dst, src []byte) ([]byte, error), callback func(data []byte) error) error {
	lr := io.LimitReader(r, maxDataSize.N+1)
	cbb := compressedBufPool.Get()

	_, err := cbb.ReadFrom(lr)
	if err != nil {
		compressedBufPool.Put(cbb)
		return fmt.Errorf("cannot read request body: %w", err)
	}
	if int64(len(cbb.B)) > maxDataSize.N {
		compressedBufPool.Put(cbb)
		return fmt.Errorf("too big compressed data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
	}

	dbb := decompressedBufPool.Get()
	defer decompressedBufPool.Put(dbb)

	dbb.B, err = decompress(dbb.B, cbb.B)
	compressedBufPool.Put(cbb)
	if err != nil {
		return fmt.Errorf("cannot decompress data: %w", err)
	}
	if int64(len(dbb.B)) > maxDataSize.N {
		return fmt.Errorf("too big decompressed data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
	}

	return callback(dbb.B)
}

var (
	compressedBufPool   bytesutil.ByteBufferPool
	decompressedBufPool bytesutil.ByteBufferPool
)

// GetUncompressedReader returns uncompressed reader for r and the given encoding
//
// The returned reader must be passed to PutUncompressedReader when no longer needed.
func GetUncompressedReader(r io.Reader, encoding string) (io.Reader, error) {
	switch encoding {
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
		return &plainReader{
			r: r,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
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
		// do nothing
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
	sr.b, err = snappy.Decode(sr.b[:cap(sr.b)], cbb.B)
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
