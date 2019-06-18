package prompb

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/golang/snappy"
)

// ReadSnappy reads r, unpacks it using snappy, appends it to dst
// and returns the result.
func ReadSnappy(dst []byte, r io.Reader, maxSize int64) ([]byte, error) {
	lr := io.LimitReader(r, maxSize+1)
	bb := bodyBufferPool.Get()
	reqLen, err := bb.ReadFrom(lr)
	if err != nil {
		bodyBufferPool.Put(bb)
		return dst, fmt.Errorf("cannot read compressed request: %s", err)
	}
	if reqLen > maxSize {
		return dst, fmt.Errorf("too big packed request; mustn't exceed %d bytes", maxSize)
	}

	buf := dst[len(dst):cap(dst)]
	buf, err = snappy.Decode(buf, bb.B)
	bodyBufferPool.Put(bb)
	if err != nil {
		err = fmt.Errorf("cannot decompress request with length %d: %s", reqLen, err)
		return dst, err
	}
	if int64(len(buf)) > maxSize {
		return dst, fmt.Errorf("too big unpacked request; musn't exceed %d bytes", maxSize)
	}
	if len(buf) > 0 && len(dst) < cap(dst) && &buf[0] == &dst[len(dst):cap(dst)][0] {
		dst = dst[:len(dst)+len(buf)]
	} else {
		dst = append(dst, buf...)
	}
	return dst, nil
}

var bodyBufferPool bytesutil.ByteBufferPool

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	for i := range wr.Timeseries {
		ts := &wr.Timeseries[i]
		ts.Labels = nil
		ts.Samples = nil
	}
	wr.Timeseries = wr.Timeseries[:0]

	for i := range wr.labelsPool {
		lb := &wr.labelsPool[i]
		lb.Name = nil
		lb.Value = nil
	}
	wr.labelsPool = wr.labelsPool[:0]

	for i := range wr.samplesPool {
		s := &wr.samplesPool[i]
		s.Value = 0
		s.Timestamp = 0
	}
	wr.samplesPool = wr.samplesPool[:0]
}
