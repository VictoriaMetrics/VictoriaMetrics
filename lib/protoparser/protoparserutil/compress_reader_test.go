package protoparserutil

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding/zstd"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestReadUncompressedData_Success(t *testing.T) {
	f := func(encoding string) {
		// Prepare data for the compression
		data := make([]byte, 64*1024)
		for i := range data {
			data[i] = byte(i)
		}

		// Compress the data with the given encoding into encodedData
		var encodedData []byte
		switch encoding {
		case "zstd":
			encodedData = zstd.CompressLevel(nil, data, 1)
		case "snappy":
			encodedData = snappy.Encode(nil, data)
		case "gzip":
			var bb bytes.Buffer
			w := gzip.NewWriter(&bb)
			if _, err := w.Write(data); err != nil {
				t.Fatalf("unexpected error when encoding gzip data: %s", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("unexpected error when closing gzip writer: %s", err)
			}
			encodedData = bb.Bytes()
		case "deflate":
			var bb bytes.Buffer
			w := zlib.NewWriter(&bb)
			if _, err := w.Write(data); err != nil {
				t.Fatalf("unexpected error when encoding zlib data: %s", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("unexpected error when closing zlib writer: %s", err)
			}
			encodedData = bb.Bytes()
		case "", "none":
			encodedData = data
		}

		r := bytes.NewBuffer(encodedData)
		maxDataLenFlag := newTestDataLenFlag(len(data))
		err := ReadUncompressedData(r, encoding, maxDataLenFlag, func(result []byte) error {
			if !bytes.Equal(result, data) {
				return fmt.Errorf("unexpected result\ngot\n%q\nwant\n%q", result, data)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error for encoding=%q: %s", encoding, err)
		}
	}

	f("zstd")
	f("snappy")
	f("gzip")
	f("deflate")
	f("")
	f("none")
}

func TestReadUncompressedData_InvalidEncoding(t *testing.T) {
	r := bytes.NewBuffer([]byte("foo bar baz"))
	encoding := "unsupported-encoding"
	maxDataLenFlag := newTestDataLenFlag(10000)
	err := ReadUncompressedData(r, encoding, maxDataLenFlag, func(result []byte) error {
		panic(fmt.Errorf("unexpected data read: %q", result))
	})
	if err == nil {
		t.Fatalf("expecting non-nil error for unsupported encoding")
	}
}

func TestReadUncompressedData_TooBigSize(t *testing.T) {
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = byte(i)
	}
	encodedData := snappy.Encode(nil, data)
	r := bytes.NewBuffer(encodedData)
	maxDataLenFlag := newTestDataLenFlag(len(data) - 1)
	err := ReadUncompressedData(r, "snappy", maxDataLenFlag, func(result []byte) error {
		panic(fmt.Errorf("unexpected dtaa read: %q", result))
	})
	if err == nil {
		t.Fatalf("expecting non-nil error for too long data")
	}
}

func newTestDataLenFlag(n int) *flagutil.Bytes {
	return &flagutil.Bytes{
		N:    int64(n),
		Name: "fake-flag",
	}
}

func TestSizeLimitedReader(t *testing.T) {
	src := strings.Repeat("a", 32)
	lr := &sizeLimitedReader{
		r:     strings.NewReader(src),
		left:  8 + 1,
		limit: 8,
		name:  "maxDecompressedRequestSize",
	}
	buf := make([]byte, 4)
	var got []byte
	var err error
	for {
		var n int
		n, err = lr.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			break
		}
	}
	if err == nil || !strings.Contains(err.Error(), "too big decompressed data size") {
		t.Fatalf("expected size error, got %v", err)
	}
	if len(got) > 9 {
		t.Fatalf("read %d bytes, want at most 9", len(got))
	}

	ok := &sizeLimitedReader{
		r:     bytes.NewReader([]byte("hello")),
		left:  5 + 1,
		limit: 5,
		name:  "maxDecompressedRequestSize",
	}
	data, err := io.ReadAll(ok)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("got %q", data)
	}
}
