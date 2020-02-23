package common

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestReadLinesBlockFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		if _, _, err := ReadLinesBlock(r, nil, nil); err == nil {
			t.Fatalf("expecting non-nil error")
		}
		sbr := &singleByteReader{
			b: []byte(s),
		}
		if _, _, err := ReadLinesBlock(sbr, nil, nil); err == nil {
			t.Fatalf("expecting non-nil error")
		}
		fr := &failureReader{}
		if _, _, err := ReadLinesBlock(fr, nil, nil); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// empty string
	f("")

	// too long string
	b := make([]byte, maxLineSize+1)
	f(string(b))
}

type failureReader struct{}

func (fr *failureReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("some error")
}

func TestReadLinesBlockMultiLinesSingleByteReader(t *testing.T) {
	f := func(s string, linesExpected []string) {
		t.Helper()

		r := &singleByteReader{
			b: []byte(s),
		}
		var err error
		var dstBuf, tailBuf []byte
		var lines []string
		for {
			dstBuf, tailBuf, err = ReadLinesBlock(r, dstBuf, tailBuf)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("unexpected error in ReadLinesBlock(%q): %s", s, err)
			}
			lines = append(lines, string(dstBuf))
		}
		if !reflect.DeepEqual(lines, linesExpected) {
			t.Fatalf("unexpected lines after reading %q: got %q; want %q", s, lines, linesExpected)
		}
	}

	f("", nil)
	f("foo", []string{"foo"})
	f("foo\n", []string{"foo"})
	f("foo\nbar", []string{"foo", "bar"})
	f("\nfoo\nbar", []string{"", "foo", "bar"})
	f("\nfoo\nbar\n", []string{"", "foo", "bar"})
	f("\nfoo\nbar\n\n", []string{"", "foo", "bar", ""})
}

func TestReadLinesBlockMultiLinesBytesBuffer(t *testing.T) {
	f := func(s string, linesExpected []string) {
		t.Helper()

		r := bytes.NewBufferString(s)
		var err error
		var dstBuf, tailBuf []byte
		var lines []string
		for {
			dstBuf, tailBuf, err = ReadLinesBlock(r, dstBuf, tailBuf)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("unexpected error in ReadLinesBlock(%q): %s", s, err)
			}
			lines = append(lines, string(dstBuf))
		}
		if !reflect.DeepEqual(lines, linesExpected) {
			t.Fatalf("unexpected lines after reading %q: got %q; want %q", s, lines, linesExpected)
		}
	}

	f("", nil)
	f("foo", []string{"foo"})
	f("foo\n", []string{"foo"})
	f("foo\nbar", []string{"foo", "bar"})
	f("\nfoo\nbar", []string{"\nfoo", "bar"})
	f("\nfoo\nbar\n", []string{"\nfoo\nbar"})
	f("\nfoo\nbar\n\n", []string{"\nfoo\nbar\n"})
}

func TestReadLinesBlockSuccessSingleByteReader(t *testing.T) {
	f := func(s, dstBufExpected, tailBufExpected string) {
		t.Helper()

		r := &singleByteReader{
			b: []byte(s),
		}
		dstBuf, tailBuf, err := ReadLinesBlock(r, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(dstBuf) != dstBufExpected {
			t.Fatalf("unexpected dstBuf; got %q; want %q; tailBuf=%q", dstBuf, dstBufExpected, tailBuf)
		}
		if string(tailBuf) != tailBufExpected {
			t.Fatalf("unexpected tailBuf; got %q; want %q; dstBuf=%q", tailBuf, tailBufExpected, dstBuf)
		}

		// Verify the same with non-empty dstBuf and tailBuf
		r = &singleByteReader{
			b: []byte(s),
		}
		dstBuf, tailBuf, err = ReadLinesBlock(r, dstBuf, tailBuf[:0])
		if err != nil {
			t.Fatalf("non-empty bufs: unexpected error: %s", err)
		}
		if string(dstBuf) != dstBufExpected {
			t.Fatalf("non-empty bufs: unexpected dstBuf; got %q; want %q; tailBuf=%q", dstBuf, dstBufExpected, tailBuf)
		}
		if string(tailBuf) != tailBufExpected {
			t.Fatalf("non-empty bufs: unexpected tailBuf; got %q; want %q; dstBuf=%q", tailBuf, tailBufExpected, dstBuf)
		}
	}

	f("\n", "", "")
	f("foo\n", "foo", "")
	f("\nfoo", "", "")
	f("foo\nbar", "foo", "")
	f("foo\nbar\nbaz", "foo", "")
	f("foo", "foo", "")

	// The maximum line size
	b := make([]byte, maxLineSize+10)
	b[maxLineSize] = '\n'
	f(string(b), string(b[:maxLineSize]), "")
}

func TestReadLinesBlockSuccessBytesBuffer(t *testing.T) {
	f := func(s, dstBufExpected, tailBufExpected string) {
		t.Helper()

		r := bytes.NewBufferString(s)
		dstBuf, tailBuf, err := ReadLinesBlock(r, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(dstBuf) != dstBufExpected {
			t.Fatalf("unexpected dstBuf; got %q; want %q; tailBuf=%q", dstBuf, dstBufExpected, tailBuf)
		}
		if string(tailBuf) != tailBufExpected {
			t.Fatalf("unexpected tailBuf; got %q; want %q; dstBuf=%q", tailBuf, tailBufExpected, dstBuf)
		}

		// Verify the same with non-empty dstBuf and tailBuf
		r = bytes.NewBufferString(s)
		dstBuf, tailBuf, err = ReadLinesBlock(r, dstBuf, tailBuf[:0])
		if err != nil {
			t.Fatalf("non-empty bufs: unexpected error: %s", err)
		}
		if string(dstBuf) != dstBufExpected {
			t.Fatalf("non-empty bufs: unexpected dstBuf; got %q; want %q; tailBuf=%q", dstBuf, dstBufExpected, tailBuf)
		}
		if string(tailBuf) != tailBufExpected {
			t.Fatalf("non-empty bufs: unexpected tailBuf; got %q; want %q; dstBuf=%q", tailBuf, tailBufExpected, dstBuf)
		}
	}

	f("\n", "", "")
	f("foo\n", "foo", "")
	f("\nfoo", "", "foo")
	f("foo\nbar", "foo", "bar")
	f("foo\nbar\nbaz", "foo\nbar", "baz")

	// The maximum line size
	b := make([]byte, maxLineSize+10)
	b[maxLineSize] = '\n'
	f(string(b), string(b[:maxLineSize]), string(b[maxLineSize+1:]))
}

type singleByteReader struct {
	b []byte
}

func (sbr *singleByteReader) Read(p []byte) (int, error) {
	if len(sbr.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, sbr.b[:1])
	sbr.b = sbr.b[n:]
	if len(sbr.b) == 0 {
		return n, io.EOF
	}
	return n, nil
}
