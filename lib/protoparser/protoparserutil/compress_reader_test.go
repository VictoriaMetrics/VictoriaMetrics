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
	for _, size := range []int{0, 7, 8, 9, 32} {
		for _, bufSize := range []int{1, 4, 16} {
			t.Run(fmt.Sprintf("size=%d/buffer=%d", size, bufSize), func(t *testing.T) {
				src := strings.NewReader(strings.Repeat("a", size))
				lr := &sizeLimitedReader{r: src, left: 8, limit: 8, name: "maxDecompressedRequestSize"}
				buf := make([]byte, bufSize)
				var got []byte
				var err error
				for err == nil {
					var n int
					n, err = lr.Read(buf)
					got = append(got, buf[:n]...)
				}
				if len(got) != min(size, 8) {
					t.Fatalf("read %d bytes; want %d", len(got), min(size, 8))
				}
				if size <= 8 {
					if err != io.EOF {
						t.Fatalf("expected EOF, got %v", err)
					}
					return
				}
				if !strings.Contains(err.Error(), "too big decompressed data size") {
					t.Fatalf("expected size error, got %v", err)
				}
				if consumed := size - src.Len(); consumed != 9 {
					t.Fatalf("consumed %d bytes; want exactly 9 including the probe", consumed)
				}
				if n, nextErr := lr.Read(buf); n != 0 || nextErr != err {
					t.Fatalf("expected sticky size error without data, got %d, %v", n, nextErr)
				}
			})
		}
	}
}

func TestOpenStreamReader(t *testing.T) {
	oldLimit := maxDecompressedRequestSize.N
	maxDecompressedRequestSize.N = 8
	defer func() { maxDecompressedRequestSize.N = oldLimit }()
	for _, encoding := range []string{"", "none", "identity", "gzip", "zstd", "deflate", "snappy"} {
		for _, size := range []int{8, 9} {
			t.Run(fmt.Sprintf("encoding=%q/size=%d", encoding, size), func(t *testing.T) {
				// An overflow newline must never complete a row in the parser.
				data := []byte(strings.Repeat("a", size-1) + "\n")
				encoded := data
				switch encoding {
				case "gzip", "deflate":
					var bb bytes.Buffer
					var w io.WriteCloser
					if encoding == "gzip" {
						w = gzip.NewWriter(&bb)
					} else {
						w = zlib.NewWriter(&bb)
					}
					if _, err := w.Write(data); err != nil {
						t.Fatal(err)
					}
					if err := w.Close(); err != nil {
						t.Fatal(err)
					}
					encoded = bb.Bytes()
				case "zstd":
					encoded = zstd.CompressLevel(nil, data, 1)
				case "snappy":
					encoded = snappy.Encode(nil, data)
				}
				r, put, err := OpenStreamReader(bytes.NewReader(encoded), encoding)
				if err != nil {
					t.Fatal(err)
				}
				defer put()
				got, err := io.ReadAll(r)
				plain := encoding == "" || encoding == "none" || encoding == "identity"
				if size <= 8 || plain {
					if err != nil || !bytes.Equal(got, data) {
						t.Fatalf("got %q, %v; want %q without an error", got, err, data)
					}
				} else if err == nil || !strings.Contains(err.Error(), "too big decompressed data size") || !bytes.Equal(got, data[:8]) {
					t.Fatalf("got %q, %v; want exactly the first 8 bytes and a size error", got, err)
				}
			})
		}
	}
}
