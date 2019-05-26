package jump

import "hash"

type crc32Hasher struct {
	crc32 hash.Hash32
}

func (h *crc32Hasher) Write(p []byte) (n int, err error) {
	return h.crc32.Write(p)
}

func (h *crc32Hasher) Sum(b []byte) []byte {
	return h.crc32.Sum(b)
}

func (h *crc32Hasher) Reset() {
	h.crc32.Reset()
}

func (h *crc32Hasher) Size() int {
	return h.crc32.Size()
}

func (h *crc32Hasher) BlockSize() int {
	return h.crc32.BlockSize()
}

func (h *crc32Hasher) Sum32() uint32 {
	return h.crc32.Sum32()
}

func (h *crc32Hasher) Sum64() uint64 {
	return uint64(h.crc32.Sum32())
}

var _ hash.Hash32 = (*crc32Hasher)(nil)
var _ hash.Hash64 = (*crc32Hasher)(nil)
