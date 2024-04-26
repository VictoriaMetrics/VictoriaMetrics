package remotewrite

import (
	"fmt"
	"math"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestGetLabelsHash_Distribution(t *testing.T) {
	f := func(bucketsCount int) {
		t.Helper()

		// Distribute itemsCount hashes returned by getLabelsHash() across bucketsCount buckets.
		itemsCount := 1_000 * bucketsCount
		m := make([]int, bucketsCount)
		var labels []prompbmarshal.Label
		for i := 0; i < itemsCount; i++ {
			labels = append(labels[:0], prompbmarshal.Label{
				Name:  "__name__",
				Value: fmt.Sprintf("some_name_%d", i),
			})
			for j := 0; j < 10; j++ {
				labels = append(labels, prompbmarshal.Label{
					Name:  fmt.Sprintf("label_%d", j),
					Value: fmt.Sprintf("value_%d_%d", i, j),
				})
			}
			h := getLabelsHash(labels)
			m[h%uint64(bucketsCount)]++
		}

		// Verify that the distribution is even
		expectedItemsPerBucket := itemsCount / bucketsCount
		for _, n := range m {
			if math.Abs(1-float64(n)/float64(expectedItemsPerBucket)) > 0.04 {
				t.Fatalf("unexpected items in the bucket for %d buckets; got %d; want around %d", bucketsCount, n, expectedItemsPerBucket)
			}
		}
	}

	f(2)
	f(3)
	f(4)
	f(5)
	f(10)
}
