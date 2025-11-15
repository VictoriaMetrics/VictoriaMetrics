//go:build !cgo

package zstd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestDecomrpessLimitedOk(t *testing.T) {
	f := func(compressedData []byte, limit int) {
		t.Helper()

		_, err := DecompressLimited(nil, compressedData, limit)
		if err != nil {
			t.Fatalf("cannot decompress data with limit=%d: %s", limit, err)
		}
	}

	var bb bytes.Buffer
	for bb.Len() < 12*128*1024 {
		fmt.Fprintf(&bb, "compress/decompress big data %d, ", bb.Len())
	}
	originData := bb.Bytes()
	// block decompression
	cd := CompressLevel(nil, originData, 0)

	// decompressed size matches block limit
	f(cd, len(originData))

	// unlimited
	f(cd, 0)

}

func TestDecompressLimitedFail(t *testing.T) {
	f := func(input []byte, limit int) {
		t.Helper()
		_, err := DecompressLimited(nil, input, limit)
		if err == nil {
			t.Errorf("unexpected nil-error for decompress with limit: %d", limit)
		}

	}

	var bb bytes.Buffer
	for bb.Len() < 12*128*1024 {
		fmt.Fprintf(&bb, "compress/decompress big data %d, ", bb.Len())
	}

	// valid input bigger than limit
	f(bb.Bytes(), 1024)

	input, err := hex.DecodeString("28b52ffd8400005ed0b209000030ecaf4412")
	if err != nil {
		t.Fatalf("BUG: unexpected hex input: %s", err)
	}
	// input with framecontent bigger than actual payload
	f(input, 512)

	// input with stream windowSize bigger than limit
	input, err = hex.DecodeString("28b52ffd04981900003030304e8da22b")
	if err != nil {
		t.Fatalf("BUG: unexpected hex input: %s", err)
	}
	f(input, 8*1e6*10)
}
