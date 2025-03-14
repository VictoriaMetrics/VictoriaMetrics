package common

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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
)

// ReadUncompressedData reads uncompressed data from r using the given encoding and then passes it to the callback.
//
// The maxDataSize limits the maximum data size, which can be read from r.
//
// The callback musn't hold references to the data after returning.
func ReadUncompressedData(r io.Reader, encoding string, maxDataSize *flagutil.Bytes, callback func(data []byte) error) error {
	reader, err := GetUncompressedReader(r, encoding)
	if err != nil {
		return err
	}
	lr := io.LimitReader(reader, maxDataSize.N+1)

	bb := dataBufPool.Get()
	defer dataBufPool.Put(bb)

	wcr := writeconcurrencylimiter.GetReader(lr)
	_, err = bb.ReadFrom(wcr)
	writeconcurrencylimiter.PutReader(wcr)
	PutUncompressedReader(reader)
	if err != nil {
		return err
	}
	if int64(len(bb.B)) > maxDataSize.N {
		return fmt.Errorf("too big data size exceeding -%s=%d bytes", maxDataSize.Name, maxDataSize.N)
	}

	return callback(bb.B)
}

var dataBufPool bytesutil.ByteBufferPool

// GetUncompressedReader returns uncompressed reader for r and the given encoding
//
// The returned reader must be passed to PutUncompressedReader when no longer needed.
func GetUncompressedReader(r io.Reader, encoding string) (io.Reader, error) {
	switch encoding {
	case "zstd":
		return getZstdReader(r), nil
	case "snappy":
		return getSnappyReader(r), nil
	case "gzip":
		return getGzipReader(r)
	case "deflate":
		return getZlibReader(r)
	case "", "none":
		return r, nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}
}

// PutUncompressedReader puts r to the pool, so it could be re-used via GetUncompressedReader()
func PutUncompressedReader(r io.Reader) {
	switch t := r.(type) {
	case *snappy.Reader:
		putSnappyReader(t)
	case *zstd.Reader:
		putZstdReader(t)
	case *gzip.Reader:
		putGzipReader(t)
	case zlib.Resetter:
		putZlibReader(t)
	}
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

func getSnappyReader(r io.Reader) *snappy.Reader {
	v := snappyReaderPool.Get()
	if v == nil {
		return snappy.NewReader(r)
	}
	zr := v.(*snappy.Reader)
	zr.Reset(r)
	return zr
}

func putSnappyReader(zr *snappy.Reader) {
	snappyReaderPool.Put(zr)
}

var snappyReaderPool sync.Pool

func getZstdReader(r io.Reader) *zstd.Reader {
	v := zstdReaderPool.Get()
	if v == nil {
		return zstd.NewReader(r)
	}
	zr := v.(*zstd.Reader)
	zr.Reset(r, nil)
	return zr
}

func putZstdReader(zr *zstd.Reader) {
	zstdReaderPool.Put(zr)
}

var zstdReaderPool sync.Pool
