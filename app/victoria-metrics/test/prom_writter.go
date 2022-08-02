package test

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/promremotewrite"

// Compress marshals and compresses wr.
func Compress(wr WriteRequest) ([]byte, error) {
	data, err := wr.Marshal()
	if err != nil {
		return nil, err
	}
	return promremotewrite.Encode(nil, data)
}
