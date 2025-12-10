package stream

import (
	"io"
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

		callback := func(_ []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
			return nil
		}

		br := benchReader{
			buf: data,
		}

		for p.Next() {
			br.offset = 0
			if err := ParseStream(&br, "", nil, callback); err != nil {
				b.Fatalf("cannot parse stream: %s", err)
			}
		}
	})
}

type benchReader struct {
	offset int
	buf    []byte
}

func (br *benchReader) Read(p []byte) (int, error) {
	n := copy(p, br.buf[br.offset:])
	br.offset += n
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}
