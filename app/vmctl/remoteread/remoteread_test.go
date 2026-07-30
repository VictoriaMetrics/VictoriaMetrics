package remoteread

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/prometheus/prometheus/tsdb/chunkenc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

func testHistogram(mul int64) *histogram.Histogram {
	return &histogram.Histogram{
		Schema:          0,
		Count:           uint64(10 * mul),
		Sum:             25.5 * float64(mul),
		ZeroThreshold:   0.001,
		ZeroCount:       uint64(2 * mul),
		PositiveSpans:   []histogram.Span{{Offset: 0, Length: 2}},
		PositiveBuckets: []int64{1 * mul, 2 * mul},
		NegativeSpans:   []histogram.Span{{Offset: 0, Length: 1}},
		NegativeBuckets: []int64{4 * mul},
	}
}

func TestConvertHistograms(t *testing.T) {
	f := func(hSamples []histogramSample, labels []prompb.Label, expected []*vm.TimeSeries) {
		t.Helper()

		tss := convertHistograms(hSamples, labels)
		if !reflect.DeepEqual(tss, expected) {
			t.Fatalf("unexpected result\ngot:\n%v\nwant:\n%v", tss, expected)
		}
	}

	// series without samples
	f(nil, []prompb.Label{{Name: "__name__", Value: "foo"}}, nil)

	// series without the metric name must be skipped
	f([]histogramSample{
		{timestamp: 1000, fh: testHistogram(1).ToFloat(nil)},
	}, []prompb.Label{{Name: "job", Value: "bar"}}, nil)

	// native histogram must be converted to _count, _sum and _bucket series
	// in the same way as VictoriaMetrics does for Prometheus remote write protocol
	labels := []prompb.Label{
		{Name: "__name__", Value: "request_duration_seconds"},
		{Name: "job", Value: "bar"},
	}
	jobLabel := []vm.LabelPair{{Name: "job", Value: "bar"}}
	bucketLabels := func(vmrange string) []vm.LabelPair {
		return []vm.LabelPair{
			{Name: "job", Value: "bar"},
			{Name: "vmrange", Value: vmrange},
		}
	}
	f([]histogramSample{
		{timestamp: 1000, fh: testHistogram(1).ToFloat(nil)},
		{timestamp: 2000, fh: testHistogram(2).ToFloat(nil)},
	}, labels, []*vm.TimeSeries{
		{
			Name:       "request_duration_seconds_count",
			LabelPairs: jobLabel,
			Timestamps: []int64{1000, 2000},
			Values:     []float64{10, 20},
		},
		{
			Name:       "request_duration_seconds_sum",
			LabelPairs: jobLabel,
			Timestamps: []int64{1000, 2000},
			Values:     []float64{25.5, 51},
		},
		{
			Name:       "request_duration_seconds_bucket",
			LabelPairs: bucketLabels("-1.000e+00...-5.000e-01"),
			Timestamps: []int64{1000, 2000},
			Values:     []float64{4, 8},
		},
		{
			Name:       "request_duration_seconds_bucket",
			LabelPairs: bucketLabels("-1.000e-03...1.000e-03"),
			Timestamps: []int64{1000, 2000},
			Values:     []float64{2, 4},
		},
		{
			Name:       "request_duration_seconds_bucket",
			LabelPairs: bucketLabels("5.000e-01...1.000e+00"),
			Timestamps: []int64{1000, 2000},
			Values:     []float64{1, 2},
		},
		{
			Name:       "request_duration_seconds_bucket",
			LabelPairs: bucketLabels("1.000e+00...2.000e+00"),
			Timestamps: []int64{1000, 2000},
			Values:     []float64{3, 6},
		},
	})
}

func TestParseHistograms(t *testing.T) {
	c := chunkenc.NewHistogramChunk()
	app, err := c.Appender()
	if err != nil {
		t.Fatalf("cannot create chunk appender: %s", err)
	}
	if _, _, _, err := app.AppendHistogram(nil, 0, 1000, testHistogram(1), true); err != nil {
		t.Fatalf("cannot append histogram: %s", err)
	}
	if _, _, _, err := app.AppendHistogram(nil, 0, 2000, testHistogram(2), true); err != nil {
		t.Fatalf("cannot append histogram: %s", err)
	}

	hSamples, err := parseHistograms(prompb.Chunk_HISTOGRAM, c.Bytes())
	if err != nil {
		t.Fatalf("cannot parse histogram chunk: %s", err)
	}
	if len(hSamples) != 2 {
		t.Fatalf("unexpected number of histogram samples; got %d; want 2", len(hSamples))
	}
	for i, expected := range []struct {
		timestamp int64
		count     float64
		sum       float64
	}{
		{timestamp: 1000, count: 10, sum: 25.5},
		{timestamp: 2000, count: 20, sum: 51},
	} {
		if hSamples[i].timestamp != expected.timestamp {
			t.Fatalf("unexpected timestamp; got %d; want %d", hSamples[i].timestamp, expected.timestamp)
		}
		if hSamples[i].fh.Count != expected.count {
			t.Fatalf("unexpected count; got %f; want %f", hSamples[i].fh.Count, expected.count)
		}
		if hSamples[i].fh.Sum != expected.sum {
			t.Fatalf("unexpected sum; got %f; want %f", hSamples[i].fh.Sum, expected.sum)
		}
	}

	// unsupported chunk encoding must return error
	if _, err := parseHistograms(prompb.Chunk_XOR, c.Bytes()); err == nil {
		t.Fatalf("expecting non-nil error for unsupported chunk encoding")
	}
}

func TestProcessResponse(t *testing.T) {
	readResp := &prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{
				Timeseries: []*prompb.TimeSeries{
					{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "cpu_usage"},
							{Name: "job", Value: "bar"},
						},
						Samples: []prompb.Sample{
							{Timestamp: 1000, Value: 1.5},
						},
					},
					{
						Labels: []prompb.Label{
							{Name: "__name__", Value: "request_duration_seconds"},
							{Name: "job", Value: "bar"},
						},
						Histograms: []prompb.Histogram{
							prompb.FromIntHistogram(1000, testHistogram(1)),
						},
					},
				},
			},
		},
	}
	data, err := proto.Marshal(readResp)
	if err != nil {
		t.Fatalf("cannot marshal ReadResponse: %s", err)
	}
	compressed := snappy.Encode(nil, data)

	var tss []*vm.TimeSeries
	err = processResponse(io.NopCloser(bytes.NewReader(compressed)), func(ts *vm.TimeSeries) error {
		tss = append(tss, ts)
		return nil
	})
	if err != nil {
		t.Fatalf("cannot process response: %s", err)
	}

	// 1 float series + _count + _sum + 4 buckets
	if len(tss) != 7 {
		t.Fatalf("unexpected number of time series; got %d; want 7", len(tss))
	}
	if tss[0].Name != "cpu_usage" || !reflect.DeepEqual(tss[0].Values, []float64{1.5}) {
		t.Fatalf("unexpected float series: %v", tss[0])
	}
	if tss[1].Name != "request_duration_seconds_count" || !reflect.DeepEqual(tss[1].Values, []float64{10}) {
		t.Fatalf("unexpected _count series: %v", tss[1])
	}
	if tss[2].Name != "request_duration_seconds_sum" || !reflect.DeepEqual(tss[2].Values, []float64{25.5}) {
		t.Fatalf("unexpected _sum series: %v", tss[2])
	}
	for _, ts := range tss[3:] {
		if ts.Name != "request_duration_seconds_bucket" {
			t.Fatalf("unexpected bucket series name %q", ts.Name)
		}
	}
}

type nopFlusher struct{}

func (nopFlusher) Flush() {}

func TestProcessStreamResponse(t *testing.T) {
	// build a histogram chunk
	hc := chunkenc.NewHistogramChunk()
	hApp, err := hc.Appender()
	if err != nil {
		t.Fatalf("cannot create histogram chunk appender: %s", err)
	}
	if _, _, _, err := hApp.AppendHistogram(nil, 0, 1000, testHistogram(1), true); err != nil {
		t.Fatalf("cannot append histogram: %s", err)
	}

	// build a float chunk
	xc := chunkenc.NewXORChunk()
	xApp, err := xc.Appender()
	if err != nil {
		t.Fatalf("cannot create xor chunk appender: %s", err)
	}
	xApp.Append(0, 1000, 1.5)

	res := &prompb.ChunkedReadResponse{
		ChunkedSeries: []*prompb.ChunkedSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "request_duration_seconds"},
					{Name: "job", Value: "bar"},
				},
				Chunks: []prompb.Chunk{
					{Type: prompb.Chunk_HISTOGRAM, Data: hc.Bytes()},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "cpu_usage"},
				},
				Chunks: []prompb.Chunk{
					{Type: prompb.Chunk_XOR, Data: xc.Bytes()},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "memory_usage"},
				},
				Chunks: []prompb.Chunk{
					// the `type` field may be unset for XOR chunks,
					// such chunks must be parsed as XOR ones
					{Type: prompb.Chunk_UNKNOWN, Data: xc.Bytes()},
				},
			},
		},
	}
	data, err := proto.Marshal(res)
	if err != nil {
		t.Fatalf("cannot marshal ChunkedReadResponse: %s", err)
	}
	var buf bytes.Buffer
	cw := remote.NewChunkedWriter(&buf, nopFlusher{})
	if _, err := cw.Write(data); err != nil {
		t.Fatalf("cannot write chunked response: %s", err)
	}

	var tss []*vm.TimeSeries
	err = processStreamResponse(io.NopCloser(&buf), func(ts *vm.TimeSeries) error {
		tss = append(tss, ts)
		return nil
	})
	if err != nil {
		t.Fatalf("cannot process stream response: %s", err)
	}

	// _count + _sum + 4 buckets + 1 float series + 1 float series from UNKNOWN chunk
	if len(tss) != 8 {
		t.Fatalf("unexpected number of time series; got %d; want 8", len(tss))
	}
	if tss[0].Name != "request_duration_seconds_count" || !reflect.DeepEqual(tss[0].Values, []float64{10}) {
		t.Fatalf("unexpected _count series: %v", tss[0])
	}
	if tss[1].Name != "request_duration_seconds_sum" || !reflect.DeepEqual(tss[1].Values, []float64{25.5}) {
		t.Fatalf("unexpected _sum series: %v", tss[1])
	}
	for _, ts := range tss[2:6] {
		if ts.Name != "request_duration_seconds_bucket" {
			t.Fatalf("unexpected bucket series name %q", ts.Name)
		}
	}
	if tss[6].Name != "cpu_usage" || !reflect.DeepEqual(tss[6].Values, []float64{1.5}) {
		t.Fatalf("unexpected float series: %v", tss[6])
	}
	if tss[7].Name != "memory_usage" || !reflect.DeepEqual(tss[7].Values, []float64{1.5}) {
		t.Fatalf("unexpected float series from UNKNOWN chunk: %v", tss[7])
	}
}

func TestParseFloatHistograms(t *testing.T) {
	c := chunkenc.NewFloatHistogramChunk()
	app, err := c.Appender()
	if err != nil {
		t.Fatalf("cannot create chunk appender: %s", err)
	}
	fh := testHistogram(1).ToFloat(nil)
	if _, _, _, err := app.AppendFloatHistogram(nil, 0, 1000, fh, true); err != nil {
		t.Fatalf("cannot append float histogram: %s", err)
	}

	hSamples, err := parseHistograms(prompb.Chunk_FLOAT_HISTOGRAM, c.Bytes())
	if err != nil {
		t.Fatalf("cannot parse float histogram chunk: %s", err)
	}
	if len(hSamples) != 1 {
		t.Fatalf("unexpected number of histogram samples; got %d; want 1", len(hSamples))
	}
	if hSamples[0].timestamp != 1000 {
		t.Fatalf("unexpected timestamp; got %d; want 1000", hSamples[0].timestamp)
	}
	if hSamples[0].fh.Count != 10 {
		t.Fatalf("unexpected count; got %f; want 10", hSamples[0].fh.Count)
	}
}
