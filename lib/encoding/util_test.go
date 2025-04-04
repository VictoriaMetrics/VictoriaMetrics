package encoding_test

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/golang/snappy"
)

func TestIsZstd(t *testing.T) {
	// nil
	if encoding.IsZstd(nil) {
		t.Fatalf("unexpected IsZstd result; got true; expecting false")
	}

	// empty
	if encoding.IsZstd([]byte{}) {
		t.Fatalf("unexpected IsZstd result; got true; expecting false")
	}

	// less than 4 bytes
	if encoding.IsZstd([]byte(`foo`)) {
		t.Fatalf("unexpected IsZstd result; got true; expecting false")
	}

	// plain text
	if encoding.IsZstd([]byte(`foobar`)) {
		t.Fatalf("unexpected IsZstd result; got true; expecting false")
	}

	// snappy compressed
	if encoding.IsZstd(snappy.Encode(nil, []byte(`foobar`))) {
		t.Fatalf("unexpected IsZstd result; got true; expecting false")
	}

	// zstd minimum compressed level
	if !encoding.IsZstd(encoding.CompressZSTDLevel(nil, []byte(`foobar`), -22)) {
		t.Fatalf("unexpected IsZstd result; got false; expecting true")
	}

	// zstd maximum compressed level
	if !encoding.IsZstd(encoding.CompressZSTDLevel(nil, []byte(`foobar`), 22)) {
		t.Fatalf("unexpected IsZstd result; got false; expecting true")
	}
}
