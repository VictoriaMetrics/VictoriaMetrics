package stream

import (
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/golang/snappy"
	"sigs.k8s.io/randfill"
)

func FuzzParseStream(f *testing.F) {
	f.Add((&pb.MetricsData{ResourceMetrics: []*pb.ResourceMetrics{
		generateOTLPSamples([]*pb.Metric{generateGauge("cpu", "")}),
	}}).MarshalProtobuf(nil))
	f.Add([]byte{0x0a, 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = ParseStream(bytes.NewReader(data), "", nil, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
			return nil
		})
	})
}

func FuzzParseStreamEncodings(f *testing.F) {
	valid := (&pb.MetricsData{ResourceMetrics: []*pb.ResourceMetrics{
		generateOTLPSamples([]*pb.Metric{generateGauge("cpu", "")}),
	}}).MarshalProtobuf(nil)
	f.Add(valid, uint8(0))
	f.Add(valid, uint8(1))
	f.Add(valid, uint8(2))
	f.Add([]byte{0x0a, 0xff}, uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, level uint8) {
		if len(data) > 4096 {
			t.Skip()
		}
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		levels := []int{1, 3, 5}
		for _, body := range [][]byte{
			buf.Bytes(),
			snappy.Encode(nil, data),
			encoding.CompressZSTDLevel(nil, data, levels[int(level)%len(levels)]),
		} {
			for _, contentEncoding := range []string{"", "gzip", "snappy", "zstd"} {
				_ = ParseStream(bytes.NewReader(body), contentEncoding, nil, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
					return nil
				})
			}
		}
	})
}

func FuzzStructuredMetrics(f *testing.F) {
	f.Add([]byte("cpu"), uint8(0))
	f.Add([]byte("cpu"), uint8(1))
	f.Add([]byte{}, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, count uint8) {
		if len(data) > 4096 {
			t.Skip()
		}
		r := randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3)
		var (
			name, unit  string
			description string
		)
		r.Fill(&name)
		r.Fill(&unit)
		r.Fill(&description)
		metric := &pb.Metric{Name: "fuzz_" + name, Unit: unit, Description: description, Gauge: &pb.Gauge{}}
		for i := 0; i < int(count%3)+1; i++ {
			var timestamp uint64
			var value *int64
			var attributes map[string]string
			r.Fill(&timestamp)
			r.Fill(&value)
			r.Fill(&attributes)
			point := &pb.NumberDataPoint{TimeUnixNano: timestamp, IntValue: value}
			for key, val := range attributes {
				point.Attributes = append(point.Attributes, stringAttributeFromKV(key, val))
			}
			metric.Gauge.DataPoints = append(metric.Gauge.DataPoints, point)
		}
		generated := (&pb.MetricsData{ResourceMetrics: []*pb.ResourceMetrics{
			generateOTLPSamples([]*pb.Metric{metric}),
		}}).MarshalProtobuf(nil)
		_ = ParseStream(bytes.NewReader(generated), "", nil, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
			return nil
		})
	})
}

func FuzzSanitizeNames(f *testing.F) {
	f.Add("http.requests.total", "s/min", "9-host", uint8(0))
	f.Add("cpu_usage", "1", "_region", uint8(1))
	f.Add("requests_total", "By/s", "__name__", uint8(0))
	f.Add("", "%", "", uint8(1))

	f.Fuzz(func(t *testing.T, name, unit, label string, typ uint8) {
		if len(name)+len(unit)+len(label) > 4096 {
			t.Skip()
		}
		var sctx sanitizerContext
		_ = sctx.sanitizePrometheusLabelName(label)
		mm := &pb.MetricMetadata{Name: name, Unit: unit, Type: prompb.MetricTypeGauge}
		if typ%2 == 0 {
			mm.Type = prompb.MetricTypeCounter
		}
		_ = sctx.sanitizePrometheusMetricName(mm)
		sctx.reset()
	})
}
