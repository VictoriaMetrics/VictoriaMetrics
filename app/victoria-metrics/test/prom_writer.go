package test

import "github.com/golang/snappy"

// Compress marshals and compresses wr.
func Compress(wr WriteRequest) ([]byte, error) {
	data, err := wr.Marshal()
	if err != nil {
		return nil, err
	}
	return snappy.Encode(nil, data), nil
}
