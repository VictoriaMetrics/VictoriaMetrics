package stream

import (
	"fmt"
	"io"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseStream(b *testing.B) {
	b.Run("default-metrics-labels-formatting", func(b *testing.B) {
		benchmarkParseStream(b, false, false)
	})
	b.Run("prometheus-metrics-labels-formatting", func(b *testing.B) {
		benchmarkParseStream(b, true, false)
	})
	b.Run("prometheus-metrics-formatting", func(b *testing.B) {
		benchmarkParseStream(b, false, true)
	})
}

func benchmarkParseStream(b *testing.B, promMetricsLabelsFormatting, promMetricsFormatting bool) {
	prevUsePrometheusNaming := *usePrometheusNaming
	prevConvertMetricNamesToPrometheus := *convertMetricNamesToPrometheus
	*usePrometheusNaming = promMetricsLabelsFormatting
	*convertMetricNamesToPrometheus = promMetricsFormatting
	defer func() {
		*usePrometheusNaming = prevUsePrometheusNaming
		*convertMetricNamesToPrometheus = prevConvertMetricNamesToPrometheus
	}()

	samples := []*pb.Metric{
		generateGauge("my-gauge", "s", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateGaugeUnknown("my-gauge-unknown", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-1", "m", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-2", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-sum-3", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-1", "m", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-2", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSum("my-counter-3", "m/s", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateSummary("my-summary", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateHistogram("my-histogram-no-sum", "", false, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateHistogram("my-histogram-with-sum", "", true, stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
		generateExpHistogram("my-exp-histogram", "", stringAttributeFromKV("job", "foo"), stringAttributeFromKV("instance", "host-123:456")),
	}

	pbRequest := pb.MetricsData{
		ResourceMetrics: []*pb.ResourceMetrics{
			generateOTLPSamples(samples),
		},
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

const (
	benchPushSampleTimestampNano = uint64(1770773711000000000)
	benchPushSampleValue         = float64(1)
	benchPushSampleFlags         = uint32(0)
)

// BenchmarkWriteRequestContextPushSample is to see how many memory is allocated (used) when handling large OTLP request.
// go test -bench=BenchmarkWriteRequestContextPushSample -benchmem -memprofile=streamparser_WriteRequestContextPushSample.out
//
// see: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10378
func BenchmarkWriteRequestContextPushSample(b *testing.B) {
	// called multiple times with different *promutil.Labels but same *wctx.
	timeSeriesCount := 100000
	labels := make([]*promutil.Labels, 0, timeSeriesCount)
	mms := make([]*pb.MetricMetadata, 0, timeSeriesCount)
	for i := range timeSeriesCount {
		lbs := &promutil.Labels{}
		for j := range 20 {
			lbs.Labels = append(lbs.Labels, prompb.Label{
				Name:  fmt.Sprintf("some_super_long_label_%d", j),
				Value: fmt.Sprintf("some_super_super_super_super_super_super_long_label_%d", j),
			})
		}
		labels = append(labels, lbs)

		mms = append(mms, &pb.MetricMetadata{
			Name: fmt.Sprintf("some_super_long_metric_name_%d", i),
			Unit: "seconds",
			Type: prompb.MetricTypeCounter,
		})
	}

	b.Run("WriteRequestContextPushSample_10k_series", func(b *testing.B) {
		wctx := getWriteRequestContext()
		defer putWriteRequestContext(wctx)

		wctx.flushFunc = func(_ []prompb.TimeSeries, _ []prompb.MetricMetadata) error {
			// let's use a no-op here.
			return nil
		}

		for i := range timeSeriesCount {
			benchmarkWriteRequestContextPushSample(b, wctx, mms[i], labels[i])
		}

		// since the flushFunc is a no-op so no need to check error response.
		_ = wctx.flushFunc(wctx.tss, wctx.mms)
	})
}

func benchmarkWriteRequestContextPushSample(b *testing.B, wctx *writeRequestContext, mm *pb.MetricMetadata, labels *promutil.Labels) {
	wctx.PushSample(mm, "", labels, benchPushSampleTimestampNano, benchPushSampleValue, benchPushSampleFlags)
}
