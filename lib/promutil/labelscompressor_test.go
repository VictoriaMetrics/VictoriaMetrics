package promutil

import (
	"fmt"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestLabelsCompressorSerial(t *testing.T) {
	var lc LabelsCompressor

	f := func(labels []prompb.Label) {
		t.Helper()

		sExpected := labelsToString(labels)

		data := lc.Compress(nil, labels)
		labelsResult := lc.Decompress(nil, data)

		sResult := labelsToString(labelsResult)
		if sExpected != sResult {
			t.Fatalf("unexpected result; got %s; want %s", sResult, sExpected)
		}

		if len(labels) > 0 {
			if n := lc.SizeBytes(); n == 0 {
				t.Fatalf("Unexpected zero SizeBytes()")
			}
			if n := lc.ItemsCount(); n == 0 {
				t.Fatalf("Unexpected zero ItemsCount()")
			}
		}
	}

	// empty labels
	f(nil)
	f([]prompb.Label{})

	// non-empty labels
	f([]prompb.Label{
		{
			Name:  "instance",
			Value: "12345.4342.342.3",
		},
		{
			Name:  "job",
			Value: "kube-pod-12323",
		},
	})
	f([]prompb.Label{
		{
			Name:  "instance",
			Value: "12345.4342.342.3",
		},
		{
			Name:  "job",
			Value: "kube-pod-12323",
		},
		{
			Name:  "pod",
			Value: "foo-bar-baz",
		},
	})
}

func TestLabelsCompressorConcurrent(t *testing.T) {
	const concurrency = 5
	var lc LabelsCompressor
	var expectCompressedKeys sync.Map

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Go(func() {
			series := newTestSeries(100, 20)
			for n, labels := range series {
				sExpected := labelsToString(labels)
				data := lc.Compress(nil, labels)
				if expectData, ok := expectCompressedKeys.LoadOrStore(n, data); ok {
					if string(data) != string(expectData.([]byte)) {
						panic(fmt.Errorf("unexpected compress result at series/%d in iteration %d ", n, i))
					}
				}
				labelsResult := lc.Decompress(nil, data)
				sResult := labelsToString(labelsResult)
				if sExpected != sResult {
					panic(fmt.Errorf("unexpected result on iteration %d; got %s; want %s", i, sResult, sExpected))
				}
			}
		})
	}
	wg.Wait()

	if n := lc.SizeBytes(); n == 0 {
		t.Fatalf("Unexpected zero SizeBytes()")
	}
	if n := lc.ItemsCount(); n == 0 {
		t.Fatalf("Unexpected zero ItemsCount()")
	}
}

func labelsToString(labels []prompb.Label) string {
	l := Labels{
		Labels: labels,
	}
	return l.String()
}

func newTestSeries(seriesCount, labelsPerSeries int) [][]prompb.Label {
	series := make([][]prompb.Label, seriesCount)
	for i := 0; i < seriesCount; i++ {
		labels := make([]prompb.Label, labelsPerSeries)
		for j := 0; j < labelsPerSeries; j++ {
			labels[j] = prompb.Label{
				Name:  fmt.Sprintf("label_%d", j),
				Value: fmt.Sprintf("value_%d_%d", i, j),
			}
		}
		series[i] = labels
	}
	return series
}
