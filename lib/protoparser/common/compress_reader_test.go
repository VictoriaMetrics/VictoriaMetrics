package common

import (
	"bytes"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"
	"io"
	"testing"
)

func TestGetAndPutCompressReader(t *testing.T) {
	str := "hello world"
	var data []byte = []byte(str)
	var bb bytes.Buffer
	gzipWriter := gzip.NewWriter(&bb)
	zlibWriter := zlib.NewWriter(&bb)
	zstdWriter, _ := zstd.NewWriter(&bb)
	snappyWriter := snappy.NewBufferedWriter(&bb)

	compressionWriter := map[string]io.WriteCloser{
		Gzip:   gzipWriter,
		Zlib:   zlibWriter,
		Zstd:   zstdWriter,
		Snappy: snappyWriter,
	}

	for compressionType, writer := range compressionWriter {
		bb.Reset()
		if _, err := writer.Write(data); err != nil {
			t.Fatalf("cannot compress data: %s", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("cannot close %s-writer: %s", compressionType, err)
		}
		r, err := GetCompressReader(compressionType, &bb)
		if err != nil {
			t.Fatalf("failed to get %s-reader: %s", compressionType, err)
		}
		err = PutCompressReader(compressionType, r)
		if err != nil {
			t.Fatalf("failed to put %s-reader: %s", compressionType, err)
		}
	}

	// invalid compression type
	_, err := GetCompressReader("invalid-compressionType", &bb)
	if err == nil {
		t.Fatalf("get invalid compression type should be failed")
	}

}
