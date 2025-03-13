package common

import (
	"fmt"
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/snappy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
)

// GetGzipReader returns new gzip reader from the pool.
//
// Return back the gzip reader when it no longer needed with PutGzipReader.
func GetGzipReader(r io.Reader) (io.ReadCloser, error) {
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

// PutGzipReader returns back gzip reader obtained via GetGzipReader.
func PutGzipReader(zr io.ReadCloser) {
	_ = zr.Close()
	gzipReaderPool.Put(zr)
}

var gzipReaderPool sync.Pool

// GetZlibReader returns zlib reader.
func GetZlibReader(r io.Reader) (io.ReadCloser, error) {
	v := zlibReaderPool.Get()
	if v == nil {
		return zlib.NewReader(r)
	}
	zr := v.(io.ReadCloser)
	if err := zr.(zlib.Resetter).Reset(r, nil); err != nil {
		return nil, err
	}
	return zr, nil
}

// PutZlibReader returns back zlib reader obtained via GetZlibReader.
func PutZlibReader(zr io.ReadCloser) {
	_ = zr.Close()
	zlibReaderPool.Put(zr)
}

var zlibReaderPool sync.Pool

// GetSnappyReader returns snappy reader.
func GetSnappyReader(r io.Reader) io.ReadCloser {
	v := snappyReaderPool.Get()
	if v == nil {
		return snappy.NewReader(r)
	}
	zr := v.(*snappy.Reader)
	zr.Reset(r)
	return zr
}

// PutSnappyReader returns back zlib reader obtained via GetSnappyReader.
func PutSnappyReader(zr io.ReadCloser) {
	_ = zr.Close()
	snappyReaderPool.Put(zr)
}

var snappyReaderPool sync.Pool

// GetZstdReader returns snappy reader.
func GetZstdReader(r io.Reader) io.ReadCloser {
	v := zstdReaderPool.Get()
	if v == nil {
		return zstd.NewReader(r)
	}
	zr := v.(*zstd.Reader)
	zr.Reset(r)
	return zr
}

// PutZstdReader returns back zlib reader obtained via GetZstdReader.
func PutZstdReader(zr io.ReadCloser) {
	_ = zr.Close()
	zstdReaderPool.Put(zr)
}

var zstdReaderPool sync.Pool

// GetUncompressedReader returns uncompressed reader for r reader
//
// The returned reader must be closed when no longer needed
func GetUncompressedReader(r io.Reader, encoding string) (io.ReadCloser, error) {
	switch encoding {
	case "zstd":
		return zstd.NewReader(r), nil
	case "snappy":
		return GetSnappyReader(r), nil
	case "gzip":
		return GetGzipReader(r)
	case "deflate":
		return GetZlibReader(r)
	case "":
		rc, ok := r.(io.ReadCloser)
		if ok {
			return rc, nil
		}
		return io.NopCloser(r), nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// PutUncompressedReader closes and puts uncompressed reader back to a respective pool
func PutUncompressedReader(r io.ReadCloser, encoding string) {
	switch encoding {
	case "snappy":
		PutSnappyReader(r)
	case "zstd":
		PutZstdReader(r)
	case "gzip":
		PutGzipReader(r)
	case "deflate":
		PutZlibReader(r)
	case "":
		_ = r.Close()
	}
}
