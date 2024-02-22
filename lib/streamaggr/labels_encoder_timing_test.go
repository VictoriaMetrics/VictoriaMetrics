package streamaggr

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkEncoderEncodeDecode(b *testing.B) {
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

	le := &labelsEncoder{}
	bb := make([]byte, 0)
	labelsTmp := make([]prompbmarshal.Label, 0)
	b.Run(fmt.Sprintf("compress-decompress=%d", len(labels)), func(b *testing.B) {
		bKey := le.encode(bb[:0], labels)
		_, err := le.decode(labelsTmp, bytesutil.ToUnsafeString(bKey))
		if err != nil {
			b.Fatalf("unexpected decode err: %s", err)
		}
	})
}
