package stream

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseMetricsStream(b *testing.B) {
	samples := []*pb.Metric{
		generateGauge("my-gauge", ""),
		generateHistogram("my-histogram", ""),
		generateSum("my-sum", "", false),
		generateSummary("my-summary", ""),
	}
	b.SetBytes(1)
	b.ReportAllocs()
	b.RunParallel(func(p *testing.PB) {
		pbRequest := pb.ExportMetricsServiceRequest{
			ResourceMetrics: []*pb.ResourceMetrics{generateOTLPSamples(samples)},
		}
		data := pbRequest.MarshalProtobuf(nil)
		contentType := "application/x-protobuf"

		for p.Next() {
			err := ParseMetricsStream(bytes.NewBuffer(data), contentType, false, nil, func(_ []prompbmarshal.TimeSeries) error {
				return nil
			})
			if err != nil {
				b.Fatalf("cannot parse stream: %s", err)
			}
		}
	})

}
