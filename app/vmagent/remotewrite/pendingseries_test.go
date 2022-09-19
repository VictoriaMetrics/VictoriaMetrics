package remotewrite

import (
	"fmt"
	"testing"

	"github.com/golang/snappy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestPushWriteRequest(t *testing.T) {
	for _, rowsCount := range []int{1, 10, 100, 1e3, 1e4} {
		t.Run(fmt.Sprintf("%d", rowsCount), func(t *testing.T) {
			testPushWriteRequest(t, rowsCount)
		})
	}
}

func testPushWriteRequest(t *testing.T, rowsCount int) {
	wr := newTestWriteRequest(rowsCount, 10)
	pushBlockLen := 0
	pushBlock := func(block []byte) {
		if pushBlockLen > 0 {
			panic(fmt.Errorf("BUG: pushBlock called multiple times; pushBlockLen=%d at first call, len(block)=%d at second call", pushBlockLen, len(block)))
		}
		pushBlockLen = len(block)
	}
	pushWriteRequest(wr, pushBlock)
	b := prompbmarshal.MarshalWriteRequest(nil, wr)
	zb := snappy.Encode(nil, b)
	maxPushBlockLen := len(zb)
	minPushBlockLen := maxPushBlockLen / 2
	if pushBlockLen < minPushBlockLen {
		t.Fatalf("unexpected block len after pushWriteRequest; got %d bytes; must be at least %d bytes", pushBlockLen, minPushBlockLen)
	}
	if pushBlockLen > maxPushBlockLen {
		t.Fatalf("unexpected block len after pushWriteRequest; got %d bytes; must be smaller or equal to %d bytes", pushBlockLen, maxPushBlockLen)
	}
}

func newTestWriteRequest(seriesCount, labelsCount int) *prompbmarshal.WriteRequest {
	var wr prompbmarshal.WriteRequest
	for i := 0; i < seriesCount; i++ {
		var labels []prompbmarshal.Label
		for j := 0; j < labelsCount; j++ {
			labels = append(labels, prompbmarshal.Label{
				Name: fmt.Sprintf("label_%d_%d", i, j),
				Value: fmt.Sprintf("value_%d_%d", i, j),
			})
		}
		wr.Timeseries = append(wr.Timeseries, prompbmarshal.TimeSeries{
			Labels: labels,
			Samples: []prompbmarshal.Sample{
				{
					Value: float64(i),
					Timestamp: 1000*int64(i),
				},
			},
		})
	}
	return &wr
}
