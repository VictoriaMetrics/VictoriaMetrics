// +build cgo

package zstd

import (
	"math/rand"
	"testing"

	pure "github.com/klauspost/compress/zstd"
	cgo "github.com/valyala/gozstd"
)

func TestCompressDecompress(t *testing.T) {
	testCrossCompressDecompress(t, []byte("a"))
	testCrossCompressDecompress(t, []byte("foobarbaz"))

	var b []byte
	for i := 0; i < 64*1024; i++ {
		b = append(b, byte(rand.Int31n(256)))
	}
	testCrossCompressDecompress(t, b)
}

func testCrossCompressDecompress(t *testing.T, b []byte) {
	testCompressDecompress(t, pureCompress, pureDecompress, b)
	testCompressDecompress(t, cgoCompress, cgoDecompress, b)
	testCompressDecompress(t, pureCompress, cgoDecompress, b)
	testCompressDecompress(t, cgoCompress, pureDecompress, b)
}

func testCompressDecompress(t *testing.T, compress compressFn, decompress decompressFn, b []byte) {
	bc, err := compress(nil, b, 5)
	if err != nil {
		t.Fatalf("unexpected error when compressing b=%x: %s", b, err)
	}
	bNew, err := decompress(nil, bc)
	if err != nil {
		t.Fatalf("unexpected error when decompressing b=%x from bc=%x: %s", b, bc, err)
	}
	if string(bNew) != string(b) {
		t.Fatalf("invalid bNew; got\n%x; expecting\n%x", bNew, b)
	}

	prefix := []byte{1, 2, 33}
	bcNew, err := compress(prefix, b, 5)
	if err != nil {
		t.Fatalf("unexpected error when compressing b=%x: %s", bcNew, err)
	}
	if string(bcNew[:len(prefix)]) != string(prefix) {
		t.Fatalf("invalid prefix for b=%x; got\n%x; expecting\n%x", b, bcNew[:len(prefix)], prefix)
	}
	if string(bcNew[len(prefix):]) != string(bc) {
		t.Fatalf("invalid prefixed bcNew for b=%x; got\n%x; expecting\n%x", b, bcNew[len(prefix):], bc)
	}

	bNew, err = decompress(prefix, bc)
	if err != nil {
		t.Fatalf("unexpected error when decompressing b=%x from bc=%x with prefix: %s", b, bc, err)
	}
	if string(bNew[:len(prefix)]) != string(prefix) {
		t.Fatalf("invalid bNew prefix when decompressing bc=%x; got\n%x; expecting\n%x", bc, bNew[:len(prefix)], prefix)
	}
	if string(bNew[len(prefix):]) != string(b) {
		t.Fatalf("invalid prefixed bNew; got\n%x; expecting\n%x", bNew[len(prefix):], b)
	}
}

type compressFn func(dst, src []byte, compressionLevel int) ([]byte, error)

func pureCompress(dst, src []byte, _ int) ([]byte, error) {
	w, err := pure.NewWriter(nil,
		pure.WithEncoderCRC(false), // Disable CRC for performance reasons.
		pure.WithEncoderLevel(pure.SpeedBestCompression))
	if err != nil {
		return nil, err
	}
	return w.EncodeAll(src, dst), nil
}

func cgoCompress(dst, src []byte, compressionLevel int) ([]byte, error) {
	return cgo.CompressLevel(dst, src, compressionLevel), nil
}

type decompressFn func(dst, src []byte) ([]byte, error)

func pureDecompress(dst, src []byte) ([]byte, error) {
	decoder, err := pure.NewReader(nil)
	if err != nil {
		return nil, err
	}
	return decoder.DecodeAll(src, dst)
}

func cgoDecompress(dst, src []byte) ([]byte, error) {
	return cgo.Decompress(dst, src)
}
