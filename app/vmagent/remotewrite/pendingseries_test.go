package remotewrite

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestPushWriteRequest(t *testing.T) {
	rowsCounts := []int{1, 10, 100, 1e3, 1e4}
	expectedBlockLensProm := []int{216, 1848, 16424, 169882, 1757876}
	expectedBlockLensVM := []int{138, 492, 3927, 34995, 288476}
	for i, rowsCount := range rowsCounts {
		expectedBlockLenProm := expectedBlockLensProm[i]
		expectedBlockLenVM := expectedBlockLensVM[i]
		t.Run(fmt.Sprintf("%d", rowsCount), func(t *testing.T) {
			testPushWriteRequest(t, rowsCount, expectedBlockLenProm, expectedBlockLenVM)
		})
	}
}

func testPushWriteRequest(t *testing.T, rowsCount, expectedBlockLenProm, expectedBlockLenVM int) {
	f := func(isVMRemoteWrite bool, expectedBlockLen int) {
		t.Helper()
		wr := newTestWriteRequest(rowsCount, 20)
		pushBlockLen := 0
		pushBlock := func(block []byte) {
			if pushBlockLen > 0 {
				panic(fmt.Errorf("BUG: pushBlock called multiple times; pushBlockLen=%d at first call, len(block)=%d at second call", pushBlockLen, len(block)))
			}
			pushBlockLen = len(block)
		}
		pushWriteRequest(wr, pushBlock, isVMRemoteWrite)
		if pushBlockLen != expectedBlockLen {
			t.Fatalf("unexpected block len for rowsCount=%d, isVMRemoteWrite=%v; got %d bytes; expecting %d bytes",
				rowsCount, isVMRemoteWrite, pushBlockLen, expectedBlockLen)
		}
	}

	// Check Prometheus remote write
	f(false, expectedBlockLenProm)

	// Check VictoriaMetrics remote write
	f(true, expectedBlockLenVM)
}

func newTestWriteRequest(seriesCount, labelsCount int) *prompbmarshal.WriteRequest {
	var wr prompbmarshal.WriteRequest
	for i := 0; i < seriesCount; i++ {
		var labels []prompbmarshal.Label
		for j := 0; j < labelsCount; j++ {
			labels = append(labels, prompbmarshal.Label{
				Name:  fmt.Sprintf("label_%d_%d", i, j),
				Value: fmt.Sprintf("value_%d_%d", i, j),
			})
		}
		wr.Timeseries = append(wr.Timeseries, prompbmarshal.TimeSeries{
			Labels: labels,
			Samples: []prompbmarshal.Sample{
				{
					Value:     float64(i),
					Timestamp: 1000 * int64(i),
				},
			},
		})
	}
	return &wr
}
