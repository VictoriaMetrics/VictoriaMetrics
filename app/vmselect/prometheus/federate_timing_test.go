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
			MetricGroup: []byte("foo_bar_?_._bazaaaa_total"),
			Tags: []storage.Tag{
				{
					Key:   []byte("instance:job"),
					Value: []byte("foobarbaz:2344"),
				},
				{
					Key:   []byte("job.name"),
					Value: []byte("aaabbbccc"),
				},
			},
		},
		Values:     []float64{112.23},
		Timestamps: []int64{1234567890},
	}

	f := func(name, escapeScheme string) {
		b.Helper()

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				var bb bytes.Buffer
				for pb.Next() {
					bb.Reset()
					WriteFederate(&bb, rs, escapeScheme)
				}
			})
		})
	}

	f("without escape", "")
	f("allow-utf-8", federateEscapeSchemeUTF8)
	f("legacy-underscore", federateEscapeSchemeUnderscore)
}
