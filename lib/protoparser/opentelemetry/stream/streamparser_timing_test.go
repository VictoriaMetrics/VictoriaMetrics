package stream

import (
	"io"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseStream(b *testing.B) {
	samples := []*pb.Metric{
		generateGauge("my-gauge", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateGaugeUnknown("my-gauge-unknown", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-1", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-2", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-3", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-1", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-2", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-3", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSummary("my-summary", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateHistogram("my-histogram-no-sum", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateHistogram("my-histogram-with-sum", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateExpHistogram("my-exp-histogram", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
	}

	pbRequest := pb.ExportMetricsServiceRequest{
		ResourceMetrics: []*pb.ResourceMetrics{generateOTLPSamples(samples)},
	}
	data := pbRequest.MarshalProtobuf(nil)

	callback := func(_ []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
		return nil
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.RunParallel(func(p *testing.PB) {
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
