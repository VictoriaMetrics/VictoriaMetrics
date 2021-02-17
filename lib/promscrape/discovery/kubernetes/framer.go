package kubernetes

import (
	"encoding/json"
	"io"
)

type jsonFrameReader struct {
	r         io.ReadCloser
	decoder   *json.Decoder
	remaining []byte
}

// NewJSONFramedReader returns an io.Reader that will decode individual JSON objects off
// of a wire.
//
// The boundaries between each frame are valid JSON objects. A JSON parsing error will terminate
// the read.
func NewJSONFramedReader(r io.ReadCloser) io.ReadCloser {
	return &jsonFrameReader{
		r:       r,
		decoder: json.NewDecoder(r),
	}
}

// ReadFrame decodes the next JSON object in the stream, or returns an error. The returned
// byte slice will be modified the next time ReadFrame is invoked and should not be altered.
func (r *jsonFrameReader) Read(data []byte) (int, error) {
	// Return whatever remaining data exists from an in progress frame
	if n := len(r.remaining); n > 0 {
		if n <= len(data) {
			data = append(data[0:0], r.remaining...)
			r.remaining = nil
			return n, nil
		}
		n = len(data)
		data = append(data[0:0], r.remaining[:n]...)
		r.remaining = r.remaining[n:]
		return n, io.ErrShortBuffer
	}

	n := len(data)
	m := json.RawMessage(data[:0])
	if err := r.decoder.Decode(&m); err != nil {
		return 0, err
	}

	// If capacity of data is less than length of the message, decoder will allocate a new slice
	// and set m to it, which means we need to copy the partial result back into data and preserve
	// the remaining result for subsequent reads.
	if len(m) > n {
		data = append(data[0:0], m[:n]...)
		r.remaining = m[n:]
		return n, io.ErrShortBuffer
	}
	return len(m), nil
}

func (r *jsonFrameReader) Close() error {
	return r.r.Close()
}
