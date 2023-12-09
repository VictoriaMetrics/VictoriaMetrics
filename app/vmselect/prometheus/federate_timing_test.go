package prometheus

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func BenchmarkFederate(b *testing.B) {
	rs := &netstorage.Result{
		MetricName: storage.MetricName{
			MetricGroup: []byte("foo_bar_bazaaaa_total"),
			Tags: []storage.Tag{
				{
					Key:   []byte("instance"),
					Value: []byte("foobarbaz:2344"),
				},
				{
					Key:   []byte("job"),
					Value: []byte("aaabbbccc"),
				},
			},
		},
		Values:     []float64{112.23},
		Timestamps: []int64{1234567890},
	}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var bb bytes.Buffer
		for pb.Next() {
			bb.Reset()
			WriteFederate(&bb, rs)
		}
	})
}
