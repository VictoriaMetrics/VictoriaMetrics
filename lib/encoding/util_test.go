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

	// zstd compressed level 1
	if !encoding.IsZstd(encoding.CompressZSTDLevel(nil, []byte(`foobar`), 1)) {
		t.Fatalf("unexpected IsZstd result; got false; expecting true")
	}

	// zstd compressed level 5
	if !encoding.IsZstd(encoding.CompressZSTDLevel(nil, []byte(`foobar`), 5)) {
		t.Fatalf("unexpected IsZstd result; got false; expecting true")
	}
}
