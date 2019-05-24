package encoding

import (
	"math/rand"
	"testing"
)

func TestCompressDecompressZSTD(t *testing.T) {
	testCompressDecompressZSTD(t, []byte("a"))
	testCompressDecompressZSTD(t, []byte("foobarbaz"))

	var b []byte
	for i := 0; i < 64*1024; i++ {
		b = append(b, byte(rand.Int31n(256)))
	}
	testCompressDecompressZSTD(t, b)
}

func testCompressDecompressZSTD(t *testing.T, b []byte) {
	bc := CompressZSTDLevel(nil, b, 5)
	bNew, err := DecompressZSTD(nil, bc)
	if err != nil {
		t.Fatalf("unexpected error when decompressing b=%x from bc=%x: %s", b, bc, err)
	}
	if string(bNew) != string(b) {
		t.Fatalf("invalid bNew; got\n%x; expecting\n%x", bNew, b)
	}

	prefix := []byte{1, 2, 33}
	bcNew := CompressZSTDLevel(prefix, b, 5)
	if string(bcNew[:len(prefix)]) != string(prefix) {
		t.Fatalf("invalid prefix for b=%x; got\n%x; expecting\n%x", b, bcNew[:len(prefix)], prefix)
	}
	if string(bcNew[len(prefix):]) != string(bc) {
		t.Fatalf("invalid prefixed bcNew for b=%x; got\n%x; expecting\n%x", b, bcNew[len(prefix):], bc)
	}

	bNew, err = DecompressZSTD(prefix, bc)
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
