package bytesutil

import (
	"bytes"
	"testing"
)

func TestResize(t *testing.T) {
	for i := 0; i < 1000; i++ {
		b := Resize(nil, i)
		if len(b) != i {
			t.Fatalf("invalid b size; got %d; expecting %d", len(b), i)
		}
		b1 := Resize(b, i)
		if len(b1) != len(b) || (len(b) > 0 && &b1[0] != &b[0]) {
			t.Fatalf("invalid b1; got %x; expecting %x", b1, b)
		}
		b2 := Resize(b[:0], i)
		if len(b2) != len(b) || (len(b) > 0 && &b2[0] != &b[0]) {
			t.Fatalf("invalid b2; got %x; expecting %x", b2, b)
		}
	}
}

func TestToUnsafeString(t *testing.T) {
	s := "str"
	if !bytes.Equal([]byte("str"), ToUnsafeBytes(s)) {
		t.Fatalf(`[]bytes(%s) doesnt equal to %s `, s, s)
	}
}
