package bytesutil

import (
	"bytes"
	"testing"
)

func TestResizeNoCopy(t *testing.T) {
	for i := 0; i < 1000; i++ {
		b := ResizeNoCopy(nil, i)
		if len(b) != i {
			t.Fatalf("invalid b size; got %d; expecting %d", len(b), i)
		}
		b1 := ResizeNoCopy(b, i)
		if len(b1) != len(b) || (len(b) > 0 && &b1[0] != &b[0]) {
			t.Fatalf("invalid b1; got %x; expecting %x", &b1[0], &b[0])
		}
		b2 := ResizeNoCopy(b[:0], i)
		if len(b2) != len(b) || (len(b) > 0 && &b2[0] != &b[0]) {
			t.Fatalf("invalid b2; got %x; expecting %x", &b2[0], &b[0])
		}
		if i > 0 {
			b[0] = 123
			b3 := ResizeNoCopy(b, i+1)
			if len(b3) != i+1 {
				t.Fatalf("invalid b3 len; got %d; want %d", len(b3), i+1)
			}
			if &b3[0] == &b[0] {
				t.Fatalf("b3 must be newly allocated")
			}
			if b3[0] != 0 {
				t.Fatalf("b3[0] must be zeroed; got %d", b3[0])
			}
		}
	}
}

func TestResizeWithCopy(t *testing.T) {
	for i := 0; i < 1000; i++ {
		b := ResizeWithCopy(nil, i)
		if len(b) != i {
			t.Fatalf("invalid b size; got %d; expecting %d", len(b), i)
		}
		b1 := ResizeWithCopy(b, i)
		if len(b1) != len(b) || (len(b) > 0 && &b1[0] != &b[0]) {
			t.Fatalf("invalid b1; got %x; expecting %x", &b1[0], &b[0])
		}
		b2 := ResizeWithCopy(b[:0], i)
		if len(b2) != len(b) || (len(b) > 0 && &b2[0] != &b[0]) {
			t.Fatalf("invalid b2; got %x; expecting %x", &b2[0], &b[0])
		}
		if i > 0 {
			b[0] = 123
			b3 := ResizeWithCopy(b, i+1)
			if len(b3) != i+1 {
				t.Fatalf("invalid b3 len; got %d; want %d", len(b3), i+1)
			}
			if &b3[0] == &b[0] {
				t.Fatalf("b3 must be newly allocated for i=%d", i)
			}
			if b3[0] != b[0] || b3[0] != 123 {
				t.Fatalf("b3[0] must equal b[0]; got %d; want %d", b3[0], b[0])
			}
		}
	}
}

func TestToUnsafeString(t *testing.T) {
	s := "str"
	if !bytes.Equal([]byte("str"), ToUnsafeBytes(s)) {
		t.Fatalf(`[]bytes(%s) doesnt equal to %s `, s, s)
	}
}
