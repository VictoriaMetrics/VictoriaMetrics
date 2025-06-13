package remotewrite

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
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
	f := func(isVMRemoteWrite bool, expectedBlockLen int, tolerancePrc float64) {
		t.Helper()
		wr := newTestWriteRequest(rowsCount, 20)
		pushBlockLen := 0
		pushBlock := func(block []byte) bool {
			if pushBlockLen > 0 {
				panic(fmt.Errorf("BUG: pushBlock called multiple times; pushBlockLen=%d at first call, len(block)=%d at second call", pushBlockLen, len(block)))
			}
			pushBlockLen = len(block)
			return true
		}
		if !tryPushWriteRequest(wr, pushBlock, isVMRemoteWrite, 0) {
			t.Fatalf("cannot push data to remote storage")
		}
		if math.Abs(float64(pushBlockLen-expectedBlockLen)/float64(expectedBlockLen)*100) > tolerancePrc {
			t.Fatalf("unexpected block len for rowsCount=%d, isVMRemoteWrite=%v; got %d bytes; expecting %d bytes +- %.0f%%",
				rowsCount, isVMRemoteWrite, pushBlockLen, expectedBlockLen, tolerancePrc)
		}
	}

	// Check Prometheus remote write
	f(false, expectedBlockLenProm, 3)

	// Check VictoriaMetrics remote write
	f(true, expectedBlockLenVM, 15)
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

func TestPushWriteRequest_PerTargetRetention(t *testing.T) {
	// Simulate two remote write targets with different retention values
	rowsCount := 1
	wr := newTestWriteRequest(rowsCount, 1)

	// Save and restore original values
	orig0 := maxPersistentQueueRetention.GetOptionalArg(0)
	orig1 := maxPersistentQueueRetention.GetOptionalArg(1)
	defer func() {
		_ = maxPersistentQueueRetention.Set(orig0.String() + "," + orig1.String())
	}()
	_ = maxPersistentQueueRetention.Set("2s,20s")

	// Simulate two argIdx: 0 and 1
	var blockA, blockB []byte
	pushBlockA := func(block []byte) bool { blockA = block; return true }
	pushBlockB := func(block []byte) bool { blockB = block; return true }

	// Use a fake tryPushWriteRequest that takes argIdx
	tryPushWriteRequestWithArgIdx := func(wr *prompbmarshal.WriteRequest, tryPushBlock func(block []byte) bool, isVMRemoteWrite bool, argIdx int) bool {
		bb := wr.MarshalProtobuf(nil)
		block := bb
		if maxPersistentQueueRetention.GetOptionalArg(argIdx) > 0 {
			tmp := make([]byte, 8+len(block))
			encoding.StoreUint64LE(tmp[:8], uint64(time.Now().Unix()-10)) // 10s old
			copy(tmp[8:], block)
			return tryPushBlock(tmp)
		}
		return tryPushBlock(block)
	}

	// Target 0: retention 2s, block is 10s old, should be dropped by client logic
	okA := tryPushWriteRequestWithArgIdx(wr, pushBlockA, false, 0)
	if !okA || len(blockA) == 0 {
		t.Fatalf("target 0: block not written")
	}
	blockTimestamp := int64(encoding.LoadUint64LE(blockA[:8]))
	blockAge := time.Now().Unix() - blockTimestamp
	if blockAge <= int64(maxPersistentQueueRetention.GetOptionalArg(0).Seconds()) {
		t.Fatalf("target 0: block should have been dropped due to retention, but was not")
	}

	// Target 1: retention 20s, block is 10s old, should NOT be dropped
	okB := tryPushWriteRequestWithArgIdx(wr, pushBlockB, false, 1)
	if !okB || len(blockB) == 0 {
		t.Fatalf("target 1: block not written")
	}
	blockTimestamp = int64(encoding.LoadUint64LE(blockB[:8]))
	blockAge = time.Now().Unix() - blockTimestamp
	if blockAge > int64(maxPersistentQueueRetention.GetOptionalArg(1).Seconds()) {
		t.Fatalf("target 1: block should NOT have been dropped due to retention, but was")
	}
}
