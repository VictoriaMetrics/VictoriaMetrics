package stream

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseStream(b *testing.B) {
	samples := []*pb.Metric{
		generateGauge("my-gauge", ""),
		generateHistogram("my-histogram", "", true),
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

		for p.Next() {
			err := ParseStream(bytes.NewBuffer(data), "", nil, func(_ []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
				return nil
			})
			if err != nil {
				b.Fatalf("cannot parse stream: %s", err)
			}
		}
	})
}
