package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"testing"
)

func BenchmarkLabelsCompressDecompress(b *testing.B) {
	labels := make([]prompbmarshal.Label, 0)
	for i := 0; i < 100; i++ {
		for j := 0; j < 1e3; j++ {
			l := prompbmarshal.Label{
				Name:  fmt.Sprintf("key%d", i),
				Value: fmt.Sprintf("value%d", j),
			}
			labels = append(labels, l)
		}
	}

	bm := &bimap{}
	bb := make([]byte, 0)
	tmpLs := promutils.GetLabels()
	b.Run(fmt.Sprintf("compress-decompress=%d", len(labels)), func(b *testing.B) {
		s := string(bm.compress(bb[:0], labels))
		tmpLs.Labels = tmpLs.Labels[:0]
		bm.decompress(tmpLs, s)
	})
}
