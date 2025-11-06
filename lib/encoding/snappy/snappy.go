package snappy

import (
	"fmt"

	"github.com/golang/snappy"
)

// Decode returns the decoded form of src with provided max block data size.
// The returned slice may be a sub-slice of dst
// if dst was large enough to hold the entire decoded block.
// Otherwise, a newly allocated slice will be returned.
//
// The dst and src must not overlap. It is valid to pass a nil dst.
//
// Decode handles the Snappy block format, not the Snappy stream format.
func Decode(dst []byte, src []byte, maxDataSizeBytes int) ([]byte, error) {
	dstLen, err := snappy.DecodedLen(src)
	if err != nil {
		return nil, fmt.Errorf("cannot read snappy header: %w", err)
	}
	if maxDataSizeBytes > 0 && dstLen > maxDataSizeBytes {
		return nil, fmt.Errorf("too big data size %d exceeding %d bytes", dstLen, maxDataSizeBytes)
	}
	return snappy.Decode(dst[:cap(dst)], src)
}
