package logstorage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func TestArena(t *testing.T) {
	values := []string{"foo", "bar", "", "adsfjkljsdfdsf", "dsfsopq", "io234"}

	for i := 0; i < 10; i++ {
		a := getArena()
		if n := len(a.b); n != 0 {
			t.Fatalf("unexpected non-zero length of empty arena: %d", n)
		}

		// add values to arena
		valuesCopy := make([]string, len(values))
		valuesLen := 0
		for j, v := range values {
			vCopy := a.copyString(v)
			if vCopy != v {
				t.Fatalf("unexpected value; got %q; want %q", vCopy, v)
			}
			valuesCopy[j] = vCopy
			valuesLen += len(v)
		}

		// verify that the values returned from arena match the original values
		for j, v := range values {
			vCopy := valuesCopy[j]
			if vCopy != v {
				t.Fatalf("unexpected value; got %q; want %q", vCopy, v)
			}
		}

		if n := len(a.b); n != valuesLen {
			t.Fatalf("unexpected arena size; got %d; want %d", n, valuesLen)
		}
		if n := a.sizeBytes(); n < valuesLen {
			t.Fatalf("unexpected arena capacity; got %d; want at least %d", n, valuesLen)
		}

		// Try allocating slices with different lengths
		bs := make([]string, 100)
		for j := range bs {
			b := a.newBytes(j)
			if len(b) != j {
				t.Fatalf("unexpected len(b); got %d; want %d", len(b), j)
			}
			valuesLen += j
			if n := len(a.b); n != valuesLen {
				t.Fatalf("unexpected arena size; got %d; want %d", n, valuesLen)
			}
			if n := a.sizeBytes(); n < valuesLen {
				t.Fatalf("unexpected arena capacity; got %d; want at least %d", n, valuesLen)
			}
			for k := range b {
				b[k] = byte(k)
			}
			bs[j] = bytesutil.ToUnsafeString(b)
		}

		// verify that the allocated slices didn't change
		for j, v := range bs {
			b := make([]byte, j)
			for k := 0; k < j; k++ {
				b[k] = byte(k)
			}
			if v != string(b) {
				t.Fatalf("unexpected value at index %d; got %X; want %X", j, v, b)
			}
		}

		// verify that the values returned from arena match the original values
		for j, v := range values {
			vCopy := valuesCopy[j]
			if vCopy != v {
				t.Fatalf("unexpected value; got %q; want %q", vCopy, v)
			}
		}

		putArena(a)
	}
}
