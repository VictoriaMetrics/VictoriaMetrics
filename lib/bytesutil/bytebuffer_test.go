package bytesutil

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestByteBuffer(t *testing.T) {
	var bb ByteBuffer

	n, err := bb.Write(nil)
	if err != nil {
		t.Fatalf("cannot write nil: %s", err)
	}
	if n != 0 {
		t.Fatalf("unexpected n when writing nil; got %d; want %d", n, 0)
	}
	if len(bb.B) != 0 {
		t.Fatalf("unexpected len(bb.B) after writing nil; got %d; want %d", len(bb.B), 0)
	}

	n, err = bb.Write([]byte{})
	if err != nil {
		t.Fatalf("cannot write empty slice: %s", err)
	}
	if n != 0 {
		t.Fatalf("unexpected n when writing empty slice; got %d; want %d", n, 0)
	}
	if len(bb.B) != 0 {
		t.Fatalf("unexpected len(bb.B) after writing empty slice; got %d; want %d", len(bb.B), 0)
	}

	data1 := []byte("123")
	n, err = bb.Write(data1)
	if err != nil {
		t.Fatalf("cannot write %q: %s", data1, err)
	}
	if n != len(data1) {
		t.Fatalf("unexpected n when writing %q; got %d; want %d", data1, n, len(data1))
	}
	if string(bb.B) != string(data1) {
		t.Fatalf("unexpected bb.B; got %q; want %q", bb.B, data1)
	}

	data2 := []byte("1")
	n, err = bb.Write(data2)
	if err != nil {
		t.Fatalf("cannot write %q: %s", data2, err)
	}
	if n != len(data2) {
		t.Fatalf("unexpected n when writing %q; got %d; want %d", data2, n, len(data2))
	}
	if string(bb.B) != string(data1)+string(data2) {
		t.Fatalf("unexpected bb.B; got %q; want %q", bb.B, string(data1)+string(data2))
	}

	bb.Reset()
	if string(bb.B) != "" {
		t.Fatalf("unexpected bb.B after reset; got %q; want %q", bb.B, "")
	}
	r := bb.NewReader().(*reader)
	if r.readOffset != 0 {
		t.Fatalf("unexpected r.readOffset after reset; got %d; want %d", r.readOffset, 0)
	}
}

func TestByteBufferReadFrom(t *testing.T) {
	var bbPool ByteBufferPool

	t.Run("zero_bytes", func(t *testing.T) {
		t.Parallel()
		bb := bbPool.Get()
		defer bbPool.Put(bb)
		src := bytes.NewBufferString("")
		n, err := bb.ReadFrom(src)
		if err != nil {
			t.Fatalf("error when reading empty string: %s", err)
		}
		if n != 0 {
			t.Fatalf("unexpected number of bytes read; got %d; want %d", n, 0)
		}
		if len(bb.B) != 0 {
			t.Fatalf("unexpejcted len(bb.B); got %d; want %d", len(bb.B), 0)
		}
	})

	t.Run("non_zero_bytes", func(t *testing.T) {
		t.Parallel()
		bb := bbPool.Get()
		defer bbPool.Put(bb)
		s := "foobarbaz"
		src := bytes.NewBufferString(s)
		n, err := bb.ReadFrom(src)
		if err != nil {
			t.Fatalf("error when reading non-empty string: %s", err)
		}
		if n != int64(len(s)) {
			t.Fatalf("unexpected number of bytes read; got %d; want %d", n, len(s))
		}
		if string(bb.B) != s {
			t.Fatalf("unexpected value read; got %q; want %q", bb.B, s)
		}
	})

	t.Run("big_number_of_bytes", func(t *testing.T) {
		t.Parallel()
		bb := bbPool.Get()
		defer bbPool.Put(bb)
		b := make([]byte, 1024*1024+234)
		for i := range b {
			b[i] = byte(i)
		}
		src := bytes.NewBuffer(b)
		n, err := bb.ReadFrom(src)
		if err != nil {
			t.Fatalf("cannot read big value: %s", err)
		}
		if n != int64(len(b)) {
			t.Fatalf("unexpected number of bytes read; got %d; want %d", n, len(b))
		}
		if string(bb.B) != string(b) {
			t.Fatalf("unexpected value read; got %q; want %q", bb.B, b)
		}
	})

	t.Run("non_empty_bb", func(t *testing.T) {
		t.Parallel()
		bb := bbPool.Get()
		defer bbPool.Put(bb)
		prefix := []byte("prefix")
		bb.B = append(bb.B[:0], prefix...)
		s := "aosdfdsafdjsf"
		src := bytes.NewBufferString(s)
		n, err := bb.ReadFrom(src)
		if err != nil {
			t.Fatalf("cannot read to non-empty bb: %s", err)
		}
		if n != int64(len(s)) {
			t.Fatalf("unexpected number of bytes read; got %d; want %d", n, len(s))
		}
		if len(bb.B) != len(prefix)+len(s) {
			t.Fatalf("unexpected bb.B len; got %d; want %d", len(bb.B), len(prefix)+len(s))
		}
		if string(bb.B[:len(prefix)]) != string(prefix) {
			t.Fatalf("unexpected prefix; got %q; want %q", bb.B[:len(prefix)], prefix)
		}
		if string(bb.B[len(prefix):]) != s {
			t.Fatalf("unexpected data read; got %q; want %q", bb.B[len(prefix):], s)
		}
	})
}

func TestByteBufferRead(t *testing.T) {
	var bb ByteBuffer

	n, err := fmt.Fprintf(&bb, "foo, %s, baz", "bar")
	if err != nil {
		t.Fatalf("unexpected error after fmt.Fprintf: %s", err)
	}
	if n != len(bb.B) {
		t.Fatalf("unexpected len(bb.B); got %d; want %d", len(bb.B), n)
	}
	if string(bb.B) != "foo, bar, baz" {
		t.Fatalf("unexpected bb.B; got %q; want %q", bb.B, "foo, bar, baz")
	}
	r := bb.NewReader().(*reader)
	if r.readOffset != 0 {
		t.Fatalf("unexpected r.readOffset; got %d; want %q", r.readOffset, 0)
	}

	rCopy := bb.NewReader().(*reader)

	var bb1 ByteBuffer
	n1, err := io.Copy(&bb1, r)
	if err != nil {
		t.Fatalf("unexpected error after io.Copy: %s", err)
	}
	if int64(r.readOffset) != n1 {
		t.Fatalf("unexpected r.readOffset after io.Copy; got %d; want %d", r.readOffset, n1)
	}
	if n1 != int64(n) {
		t.Fatalf("unexpected number of bytes copied; got %d; want %d", n1, n)
	}
	if string(bb1.B) != "foo, bar, baz" {
		t.Fatalf("unexpected bb1.B; got %q; want %q", bb1.B, "foo, bar, baz")
	}

	// Make read returns io.EOF
	buf := make([]byte, n)
	n2, err := r.Read(buf)
	if err != io.EOF {
		t.Fatalf("unexpected error returned: got %q; want %q", err, io.EOF)
	}
	if n2 != 0 {
		t.Fatalf("unexpected n1 returned; got %d; want %d", n2, 0)
	}

	// Read data from rCopy
	if rCopy.readOffset != 0 {
		t.Fatalf("unexpected rCopy.readOffset; got %d; want %d", rCopy.readOffset, 0)
	}
	buf = make([]byte, n+13)
	n2, err = rCopy.Read(buf)
	if err != io.EOF {
		t.Fatalf("unexpected error when reading from rCopy: got %q; want %q", err, io.EOF)
	}
	if n2 != n {
		t.Fatalf("unexpected number of bytes read from rCopy; got %d; want %d", n2, n)
	}
	if string(buf[:n2]) != "foo, bar, baz" {
		t.Fatalf("unexpected data read: got %q; want %q", buf[:n2], "foo, bar, baz")
	}
	if rCopy.readOffset != n2 {
		t.Fatalf("unexpected rCopy.readOffset; got %d; want %d", rCopy.readOffset, n2)
	}
}

func TestByteBufferMustReadAt(t *testing.T) {
	testStr := "foobar baz"

	var bb ByteBuffer
	bb.B = append(bb.B, testStr...)

	// Try reading at negative offset
	p := make([]byte, 1)
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expecting non-nil error when reading at negative offset")
			}
		}()
		bb.MustReadAt(p, -1)
	}()

	// Try reading past the end of buffer
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expecting non-nil error when reading past the end of buffer")
			}
		}()
		bb.MustReadAt(p, int64(len(testStr))+1)
	}()

	// Try reading the first byte
	n := len(p)
	bb.MustReadAt(p, 0)
	if string(p) != testStr[:n] {
		t.Fatalf("unexpected value read: %q; want %q", p, testStr[:n])
	}

	// Try reading the last byte
	bb.MustReadAt(p, int64(len(testStr))-1)
	if string(p) != testStr[len(testStr)-1:] {
		t.Fatalf("unexpected value read: %q; want %q", p, testStr[len(testStr)-1:])
	}

	// Try reading non-full p
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expecting non-nil error when reading non-full p")
			}
		}()
		p := make([]byte, 10)
		bb.MustReadAt(p, int64(len(testStr))-3)
	}()

	// Try reading multiple bytes from the middle
	p = make([]byte, 3)
	bb.MustReadAt(p, 2)
	if string(p) != testStr[2:2+len(p)] {
		t.Fatalf("unexpected value read: %q; want %q", p, testStr[2:2+len(p)])
	}
}

func TestByteBufferMustReadAtParallel(t *testing.T) {
	ch := make(chan error, 10)
	var bb ByteBuffer
	bb.B = []byte("foo bar baz adsf adsf dsakjlkjlkj2l34324")
	for i := 0; i < cap(ch); i++ {
		go func() {
			p := make([]byte, 3)
			for i := 0; i < len(bb.B)-len(p); i++ {
				bb.MustReadAt(p, int64(i))
			}
			ch <- nil
		}()
	}

	for i := 0; i < cap(ch); i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout")
		}
	}
}
