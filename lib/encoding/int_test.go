package encoding

import (
	"bytes"
	"fmt"
	"testing"
)

func TestMarshalUnmarshalUint16(t *testing.T) {
	testMarshalUnmarshalUint16(t, 0)
	testMarshalUnmarshalUint16(t, 1)
	testMarshalUnmarshalUint16(t, (1<<16)-1)
	testMarshalUnmarshalUint16(t, (1<<15)+1)
	testMarshalUnmarshalUint16(t, (1<<15)-1)
	testMarshalUnmarshalUint16(t, 1<<15)

	for i := uint16(0); i < 1e4; i++ {
		testMarshalUnmarshalUint16(t, i)
	}
}

func testMarshalUnmarshalUint16(t *testing.T, u uint16) {
	t.Helper()

	b := MarshalUint16(nil, u)
	if len(b) != 2 {
		t.Fatalf("unexpected b length: %d; expecting %d", len(b), 2)
	}
	uNew := UnmarshalUint16(b)
	if uNew != u {
		t.Fatalf("unexpected uNew from b=%x; got %d; expecting %d", b, uNew, u)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalUint16(prefix, u)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for u=%d; got\n%x; expecting\n%x", u, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for u=%d; got\n%x; expecting\n%x", u, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalUint32(t *testing.T) {
	testMarshalUnmarshalUint32(t, 0)
	testMarshalUnmarshalUint32(t, 1)
	testMarshalUnmarshalUint32(t, (1<<32)-1)
	testMarshalUnmarshalUint32(t, (1<<31)+1)
	testMarshalUnmarshalUint32(t, (1<<31)-1)
	testMarshalUnmarshalUint32(t, 1<<31)

	for i := uint32(0); i < 1e4; i++ {
		testMarshalUnmarshalUint32(t, i)
	}
}

func testMarshalUnmarshalUint32(t *testing.T, u uint32) {
	t.Helper()

	b := MarshalUint32(nil, u)
	if len(b) != 4 {
		t.Fatalf("unexpected b length: %d; expecting %d", len(b), 4)
	}
	uNew := UnmarshalUint32(b)
	if uNew != u {
		t.Fatalf("unexpected uNew from b=%x; got %d; expecting %d", b, uNew, u)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalUint32(prefix, u)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for u=%d; got\n%x; expecting\n%x", u, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for u=%d; got\n%x; expecting\n%x", u, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalUint64(t *testing.T) {
	testMarshalUnmarshalUint64(t, 0)
	testMarshalUnmarshalUint64(t, 1)
	testMarshalUnmarshalUint64(t, (1<<64)-1)
	testMarshalUnmarshalUint64(t, (1<<63)+1)
	testMarshalUnmarshalUint64(t, (1<<63)-1)
	testMarshalUnmarshalUint64(t, 1<<63)

	for i := uint64(0); i < 1e4; i++ {
		testMarshalUnmarshalUint64(t, i)
	}
}

func testMarshalUnmarshalUint64(t *testing.T, u uint64) {
	t.Helper()

	b := MarshalUint64(nil, u)
	if len(b) != 8 {
		t.Fatalf("unexpected b length: %d; expecting %d", len(b), 8)
	}
	uNew := UnmarshalUint64(b)
	if uNew != u {
		t.Fatalf("unexpected uNew from b=%x; got %d; expecting %d", b, uNew, u)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalUint64(prefix, u)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for u=%d; got\n%x; expecting\n%x", u, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for u=%d; got\n%x; expecting\n%x", u, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalInt16(t *testing.T) {
	testMarshalUnmarshalInt16(t, 0)
	testMarshalUnmarshalInt16(t, 1)
	testMarshalUnmarshalInt16(t, -1)
	testMarshalUnmarshalInt16(t, -1<<15)
	testMarshalUnmarshalInt16(t, (-1<<15)+1)
	testMarshalUnmarshalInt16(t, (1<<15)-1)

	for i := int16(0); i < 1e4; i++ {
		testMarshalUnmarshalInt16(t, i)
		testMarshalUnmarshalInt16(t, -i)
	}
}

func testMarshalUnmarshalInt16(t *testing.T, v int16) {
	t.Helper()

	b := MarshalInt16(nil, v)
	if len(b) != 2 {
		t.Fatalf("unexpected b length: %d; expecting %d", len(b), 2)
	}
	vNew := UnmarshalInt16(b)
	if vNew != v {
		t.Fatalf("unexpected vNew from b=%x; got %d; expecting %d", b, vNew, v)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalInt16(prefix, v)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for v=%d; got\n%x; expecting\n%x", v, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for v=%d; got\n%x; expecting\n%x", v, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalInt64(t *testing.T) {
	testMarshalUnmarshalInt64(t, 0)
	testMarshalUnmarshalInt64(t, 1)
	testMarshalUnmarshalInt64(t, -1)
	testMarshalUnmarshalInt64(t, -1<<63)
	testMarshalUnmarshalInt64(t, (-1<<63)+1)
	testMarshalUnmarshalInt64(t, (1<<63)-1)

	for i := int64(0); i < 1e4; i++ {
		testMarshalUnmarshalInt64(t, i)
		testMarshalUnmarshalInt64(t, -i)
	}
}

func testMarshalUnmarshalInt64(t *testing.T, v int64) {
	t.Helper()

	b := MarshalInt64(nil, v)
	if len(b) != 8 {
		t.Fatalf("unexpected b length: %d; expecting %d", len(b), 8)
	}
	vNew := UnmarshalInt64(b)
	if vNew != v {
		t.Fatalf("unexpected vNew from b=%x; got %d; expecting %d", b, vNew, v)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalInt64(prefix, v)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for v=%d; got\n%x; expecting\n%x", v, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for v=%d; got\n%x; expecting\n%x", v, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalVarInt64(t *testing.T) {
	testMarshalUnmarshalVarInt64(t, 0)
	testMarshalUnmarshalVarInt64(t, 1)
	testMarshalUnmarshalVarInt64(t, -1)
	testMarshalUnmarshalVarInt64(t, -1<<63)
	testMarshalUnmarshalVarInt64(t, (-1<<63)+1)
	testMarshalUnmarshalVarInt64(t, (1<<63)-1)

	for i := int64(0); i < 1e4; i++ {
		testMarshalUnmarshalVarInt64(t, i)
		testMarshalUnmarshalVarInt64(t, -i)
		testMarshalUnmarshalVarInt64(t, i<<8)
		testMarshalUnmarshalVarInt64(t, -i<<8)
		testMarshalUnmarshalVarInt64(t, i<<16)
		testMarshalUnmarshalVarInt64(t, -i<<16)
		testMarshalUnmarshalVarInt64(t, i<<23)
		testMarshalUnmarshalVarInt64(t, -i<<23)
		testMarshalUnmarshalVarInt64(t, i<<33)
		testMarshalUnmarshalVarInt64(t, -i<<33)
		testMarshalUnmarshalVarInt64(t, i<<43)
		testMarshalUnmarshalVarInt64(t, -i<<43)
		testMarshalUnmarshalVarInt64(t, i<<53)
		testMarshalUnmarshalVarInt64(t, -i<<53)
	}
}

func testMarshalUnmarshalVarInt64(t *testing.T, v int64) {
	t.Helper()

	b := MarshalVarInt64(nil, v)
	tail, vNew, err := UnmarshalVarInt64(b)
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling v=%d from b=%x: %s", v, b, err)
	}
	if vNew != v {
		t.Fatalf("unexpected vNew from b=%x; got %d; expecting %d", b, vNew, v)
	}
	if len(tail) > 0 {
		t.Fatalf("unexpected data left after unmarshaling v=%d from b=%x: %x", v, b, tail)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalVarInt64(prefix, v)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for v=%d; got\n%x; expecting\n%x", v, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for v=%d; got\n%x; expecting\n%x", v, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalVarUint64(t *testing.T) {
	testMarshalUnmarshalVarUint64(t, 0)
	testMarshalUnmarshalVarUint64(t, 1)
	testMarshalUnmarshalVarUint64(t, (1<<63)-1)

	for i := uint64(0); i < 1024; i++ {
		testMarshalUnmarshalVarUint64(t, i)
		testMarshalUnmarshalVarUint64(t, i<<8)
		testMarshalUnmarshalVarUint64(t, i<<16)
		testMarshalUnmarshalVarUint64(t, i<<23)
		testMarshalUnmarshalVarUint64(t, i<<33)
		testMarshalUnmarshalVarUint64(t, i<<41)
		testMarshalUnmarshalVarUint64(t, i<<49)
		testMarshalUnmarshalVarUint64(t, i<<54)
	}
}

func testMarshalUnmarshalVarUint64(t *testing.T, u uint64) {
	t.Helper()

	b := MarshalVarUint64(nil, u)
	tail, uNew, err := UnmarshalVarUint64(b)
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling u=%d from b=%x: %s", u, b, err)
	}
	if uNew != u {
		t.Fatalf("unexpected uNew from b=%x; got %d; expecting %d", b, uNew, u)
	}
	if len(tail) > 0 {
		t.Fatalf("unexpected data left after unmarshaling u=%d from b=%x: %x", u, b, tail)
	}

	prefix := []byte{1, 2, 3}
	b1 := MarshalVarUint64(prefix, u)
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for u=%d; got\n%x; expecting\n%x", u, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for u=%d; got\n%x; expecting\n%x", u, b1[len(prefix):], b)
	}
}

func TestMarshalUnmarshalBytes(t *testing.T) {
	testMarshalUnmarshalBytes(t, "")
	testMarshalUnmarshalBytes(t, "x")
	testMarshalUnmarshalBytes(t, "xy")

	var bb bytes.Buffer
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&bb, " %d ", i)
		s := bb.String()
		testMarshalUnmarshalBytes(t, s)
	}
}

func testMarshalUnmarshalBytes(t *testing.T, s string) {
	t.Helper()

	b := MarshalBytes(nil, []byte(s))
	tail, bNew, err := UnmarshalBytes(b)
	if err != nil {
		t.Fatalf("unexpected error when unmarshaling s=%q from b=%x: %s", s, b, err)
	}
	if string(bNew) != s {
		t.Fatalf("unexpected sNew from b=%x; got %q; expecting %q", b, bNew, s)
	}
	if len(tail) > 0 {
		t.Fatalf("unexepcted data left after unmarshaling s=%q from b=%x: %x", s, b, tail)
	}

	prefix := []byte("abcde")
	b1 := MarshalBytes(prefix, []byte(s))
	if string(b1[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix for s=%q; got\n%x; expecting\n%x", s, b1[:len(prefix)], prefix)
	}
	if string(b1[len(prefix):]) != string(b) {
		t.Fatalf("unexpected b for s=%q; got\n%x; expecting\n%x", s, b1[len(prefix):], b)
	}
}
