package test

import (
	"fmt"

	"github.com/golang/snappy"
)

// Compress marshals and compresses wr.
func Compress(wr WriteRequest) []byte {
	data, err := wr.Marshal()
	if err != nil {
		panic(fmt.Errorf("BUG: cannot compress WriteRequest: %s", err))
	}
	return snappy.Encode(nil, data)
}
