package common

import (
	"io"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
)

// GetGzipReader returns new gzip reader from the pool.
//
// Return back the gzip reader when it no longer needed with PutGzipReader.
func GetGzipReader(r io.Reader) (*gzip.Reader, error) {
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
func PutGzipReader(zr *gzip.Reader) {
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

// GetZstdReader returns zstd reader.
func GetZstdReader(r io.Reader) (zstd.Reader, error) {
	v := zstdReaderPool.Get()
	if v == nil {
		return zstd.NewReader(r)
	}
	zr := v.(zstd.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

// PutZstdReader returns back zstd reader obtained via GetZstdReader.
func PutZstdReader(zr zstd.Reader) {
	zr.Close()
	zstdReaderPool.Put(zr)
}

var zstdReaderPool sync.Pool
