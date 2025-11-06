package snappy

import (
	"encoding/hex"
	"testing"

	"github.com/golang/snappy"
	"github.com/google/go-cmp/cmp"
)

func TestDecodeOk(t *testing.T) {
	f := func(src []byte, want []byte, maxMemoryLimit int) {
		t.Helper()
		got, err := Decode(nil, src, maxMemoryLimit)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if diff := cmp.Diff(string(want), string(got)); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}
	// regular block, no limit
	data := make([]byte, 32*1024)
	encoded := snappy.Encode(nil, data)
	f(encoded, data, 0)

	// regular block, fits limit
	f(encoded, data, 68*1024)
}

func TestDecodeFail(t *testing.T) {
	f := func(src []byte, maxMemoryLimit int) {
		t.Helper()
		_, err := Decode(nil, src, maxMemoryLimit)
		if err == nil {
			t.Fatal("unexpected empty error")
		}
	}
	// mailformed block
	mailformed, err := hex.DecodeString("97eab4890a170a085f5f6e616d655f5f120b746573745f6d6574726963121009000000000000f03f10d48fc9b2a333")
	if err != nil {
		t.Fatalf("BUG: unexpected hex encoded input: %s", err)
	}
	f(mailformed, 32*1024*1024)

	// valid block exceeds maxMemoryLimit
	data := make([]byte, 32*1024)
	encoded := snappy.Encode(nil, data)
	f(encoded, 1024)

	// invalid block
	f(nil, 0)
}
