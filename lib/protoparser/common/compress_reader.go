package common

import (
	"fmt"
	"github.com/golang/snappy"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"
	"io"
	"sync"
)

// supported compression
var (
	Gzip    = "gzip"
	Deflate = "deflate"
	Zstd    = "zstd"
	Snappy  = "snappy"
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
func GetZstdReader(r io.Reader) (*zstd.Decoder, error) {
	v := zstdReaderPool.Get()
	if v == nil {
		return zstd.NewReader(r)

	}
	zd := v.(*zstd.Decoder)
	if err := zd.Reset(r); err != nil {
		return nil, err
	}
	return zd, nil
}

// PutZstdReader returns back zstd reader obtained via GetZstdReader.
func PutZstdReader(zd *zstd.Decoder) {
	_ = zd.Reset(nil)
	zstdReaderPool.Put(zd)
}

var zstdReaderPool sync.Pool

// GetSnappyReader returns snappy reader.
func GetSnappyReader(r io.Reader) (*snappy.Reader, error) {
	v := snappyReaderPool.Get()
	if v == nil {
		return snappy.NewReader(r), nil

	}
	zr := v.(*snappy.Reader)
	zr.Reset(r)
	return zr, nil
}

// PutSnappyReader returns back zstd reader obtained via GetZstdReader.
func PutSnappyReader(sr *snappy.Reader) {
	sr.Reset(nil)
	snappyReaderPool.Put(sr)
}

var snappyReaderPool sync.Pool

// GetCompressReader returns specific compression reader
func GetCompressReader(contentEncoding string, r io.Reader) (io.Reader, error) {
	switch contentEncoding {
	case Gzip:
		return GetGzipReader(r)
	case Deflate:
		return GetZlibReader(r)
	case Zstd:
		return GetZstdReader(r)
	case Snappy:
		return GetSnappyReader(r)
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", contentEncoding)
	}
}

// PutCompressReader returns specific compression reader to pool
func PutCompressReader(contentEncoding string, r io.Reader) error {
	switch contentEncoding {
	case Gzip:
		PutGzipReader(r.(*gzip.Reader))
	case Deflate:
		PutZlibReader(r.(io.ReadCloser))
	case Zstd:
		PutZstdReader(r.(*zstd.Decoder))
	case Snappy:
		PutSnappyReader(r.(*snappy.Reader))
	default:
		return fmt.Errorf("unsupported compression type: %s", contentEncoding)
	}
	return nil
}
