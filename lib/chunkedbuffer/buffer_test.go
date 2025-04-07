package chunkedbuffer

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestBuffer(t *testing.T) {
	cb := Get()
	defer Put(cb)

	for i := 0; i < 10; i++ {
		cb.Reset()

		// Write data to chunked buffer
		totalSize := 0
		for j := 1; j < 1000; j++ {
			b := make([]byte, j)
			for k := range b {
				b[k] = byte(k)
			}
			cb.MustWrite(b)
			totalSize += len(b)
		}

		cbLen := cb.Len()
		if cbLen != totalSize {
			t.Fatalf("nexpected Buffer.Len value; got %d; want %d", cbLen, totalSize)
		}

		size := cb.SizeBytes()
		if size < totalSize {
			t.Fatalf("too small SizeBytes; got %d; want at least %d", size, totalSize)
		}

		// Read the data from chunked buffer via MustReadAt.
		off := 0
		for j := 1; j < 1000; j++ {
			b := make([]byte, j)
			cb.MustReadAt(b, int64(off))
			off += j

			// Verify the data is read correctly
			for k := range b {
				if b[k] != byte(k) {
					t.Fatalf("unexpected byte read; got %d; want %d", b[k], k)
				}
			}
		}

		// Read the data from chunked buffer via NewReader.
		r := cb.NewReader()
		var bb bytes.Buffer
		n, err := bb.ReadFrom(r)
		if err != nil {
			t.Fatalf("error when reading data from chunked buffer: %s", err)
		}
		if n != int64(off) {
			t.Fatalf("unexpected amounts of data read from chunked buffer; got %d; want %d", n, off)
		}

		// Verify that reader path is equivalent to cb path
		cbPath := cb.Path()
		rPath := r.Path()
		if cbPath != rPath {
			t.Fatalf("unexpected path; got %q; want %q", rPath, cbPath)
		}

		r.MustClose()

		// Verify the read data
		off = 0
		data := bb.Bytes()
		for j := 1; j < 1000; j++ {
			b := data[off : off+j]
			off += j

			// Verify the data is read correctly
			for k := range b {
				if b[k] != byte(k) {
					t.Fatalf("unexpected byte read; got %d; want %d", b[k], k)
				}
			}
		}

		// Copy the data to another chunked buffer via WriteTo.
		cb2 := Get()
		n, err = cb.WriteTo(cb2)
		if err != nil {
			t.Fatalf("error when writing data to another chunked buffer: %s", err)
		}
		if n != int64(off) {
			t.Fatalf("unexpected amounts of data written to chunked buffer; got %d; want %d", n, off)
		}

		// Verify that the data at cb is equivalent to the data at cb2
		var bb2 bytes.Buffer
		r2 := cb2.NewReader()
		n, err = bb2.ReadFrom(r2)
		if err != nil {
			t.Fatalf("cannot read data from chunked buffer: %s", err)
		}
		if n != int64(off) {
			t.Fatalf("unexpected amounts of data read from the chunked buffer; got %d; want %d", n, off)
		}
		data2 := bb2.Bytes()

		if !bytes.Equal(data, data2) {
			t.Fatalf("unexpected data at the second chunked buffer\ngot\n%q\nwant\n%q", data2, data)
		}

		// Verify MustClose at chunked buffer
		cb2.MustClose()

		Put(cb2)
	}
}

func TestBuffer_ReadFrom(t *testing.T) {
	cb := Get()
	defer Put(cb)

	bb := bytes.NewBufferString("foo")
	n, err := cb.ReadFrom(bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if n != 3 {
		t.Fatalf("unexpected number of bytes written: %d; want 3", n)
	}

	bb = bytes.NewBufferString("bar")
	n, err = cb.ReadFrom(bb)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if n != 3 {
		t.Fatalf("unexpected number of bytes written: %d; want 3", n)
	}

	var bbResult bytes.Buffer
	cb.MustWriteTo(&bbResult)

	result := bbResult.String()
	resultExpected := "foobar"
	if result != resultExpected {
		t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
	}
}

func TestBuffer_MustReadAtZeroData(_ *testing.T) {
	var cb Buffer
	cb.MustReadAt(nil, 0)
}

func TestBuffer_ReaderZeroData(t *testing.T) {
	var cb Buffer
	r := cb.NewReader()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(data) != 0 {
		t.Fatalf("unexpected data read with len=%d; data=%q", len(data), data)
	}
}

func TestBuffer_ReaderSingleChunk(t *testing.T) {
	var cb Buffer

	fmt.Fprintf(&cb, "foo bar baz")
	r := cb.NewReader()
	b := make([]byte, 4)

	if _, err := io.ReadFull(r, b); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(b) != "foo " {
		t.Fatalf("unexpected data read; got %q; want %q", b, "foo ")
	}

	if _, err := io.ReadFull(r, b); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(b) != "bar " {
		t.Fatalf("unexpected data read; got %q; want %q", b, "bar ")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if string(data) != "baz" {
		t.Fatalf("unexpected data read; got %q; want %q", b, "baz")
	}
}

func TestBuffer_WriteToZeroData(t *testing.T) {
	var cb Buffer
	var bb bytes.Buffer
	cb.MustWriteTo(&bb)
	if bbLen := bb.Len(); bbLen != 0 {
		t.Fatalf("unexpected data written to bb with len=%d; data=%q", bbLen, bb.Bytes())
	}
}

func TestBuffer_WriteToBrokenWriter(t *testing.T) {
	var cb Buffer

	fmt.Fprintf(&cb, "foo bar baz")

	w := &faultyWriter{}
	n, err := cb.WriteTo(w)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if n != 0 {
		t.Fatalf("expecting zero bytes written; got %d bytes", n)
	}

	w = &faultyWriter{
		bytesToAccept: 5,
	}
	n, err = cb.WriteTo(w)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if n != int64(w.bytesToAccept) {
		t.Fatalf("unexpected number of bytes written; got %d; want %d", n, w.bytesToAccept)
	}

	w = &faultyWriter{
		returnInvalidBytesRead: true,
	}
	n, err = cb.WriteTo(w)
	if err == nil {
		t.Fatalf("expecting non-nil error")
	}
	if n != 0 {
		t.Fatalf("unexpected number of bytes written; got %d; want %d", n, 0)
	}
}

type faultyWriter struct {
	bytesToAccept          int
	returnInvalidBytesRead bool

	bytesRead int
}

func (fw *faultyWriter) Write(p []byte) (int, error) {
	if fw.returnInvalidBytesRead {
		return 0, nil
	}

	if fw.bytesRead+len(p) > fw.bytesToAccept {
		n := fw.bytesToAccept - fw.bytesRead
		fw.bytesRead = fw.bytesToAccept
		return n, fmt.Errorf("some error")
	}
	fw.bytesRead += len(p)
	return len(p), nil
}
