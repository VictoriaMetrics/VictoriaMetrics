package stream

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseStream(b *testing.B) {
	samples := []*pb.Metric{
		generateGauge("my-gauge"),
		generateHistogram("my-histogram"),
		generateSum("my-sum"),
		generateSummary("my-summary"),
	}
	b.SetBytes(1)
	b.ReportAllocs()
	b.RunParallel(func(p *testing.PB) {
		pbRequest := pb.ExportMetricsServiceRequest{
			ResourceMetrics: []*pb.ResourceMetrics{generateOTLPSamples(samples)},
		}
		data, err := pbRequest.MarshalVT()
		if err != nil {
			b.Fatalf("cannot marshal data: %s", err)
		}

		for p.Next() {
			err := ParseStream(bytes.NewBuffer(data), false, func(tss []prompbmarshal.TimeSeries) error {
				return nil
			})
			if err != nil {
				b.Fatalf("cannot parse stream: %s", err)
			}
		}
	})

}
