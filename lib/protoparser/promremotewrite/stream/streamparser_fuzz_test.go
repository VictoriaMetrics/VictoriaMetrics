package stream

import (
	"bytes"
	"testing"

	"github.com/golang/snappy"
	"sigs.k8s.io/randfill"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func FuzzParse(f *testing.F) {
	wr := &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{{
		Labels:  []prompb.Label{{Name: "__name__", Value: "cpu"}},
		Samples: []prompb.Sample{{Value: 1, Timestamp: 42}},
	}}}
	f.Add(snappy.Encode(nil, wr.MarshalProtobuf(nil)))
	f.Add(encoding.CompressZSTDLevel(nil, wr.MarshalProtobuf(nil), 1))
	f.Add([]byte{0xff, 0x00})
	f.Add([]byte("(\xb5/\xfd\xd3000000000000\xd3C000"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), true, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
			return nil
		})
		_ = Parse(bytes.NewReader(data), false, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
			return nil
		})
	})
}

func FuzzStructuredWriteRequest(f *testing.F) {
	f.Add([]byte("cpu"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var generated prompb.WriteRequest
		randfill.NewFromGoFuzz(data).MaxDepth(5).NilChance(0.2).NumElements(0, 3).Funcs(
			func(ts *prompb.TimeSeries, c randfill.Continue) {
				c.FillNoCustom(ts)
				ts.Labels = append(ts.Labels, prompb.Label{Name: "__name__", Value: "fuzz_" + c.String(65535)})
			},
			func(mm *prompb.MetricMetadata, c randfill.Continue) {
				c.FillNoCustom(mm)
				mm.Type %= prompb.MetricTypeStateset + 1
				mm.MetricFamilyName = "fuzz_" + c.String(65535)
			},
		).Fill(&generated)
		message := generated.MarshalProtobuf(nil)
		for _, body := range [][]byte{
			snappy.Encode(nil, message),
			encoding.CompressZSTDLevel(nil, message, 1),
		} {
			// Exercise both codec selections and their queue-compatibility fallbacks.
			for _, isVMRemoteWrite := range []bool{false, true} {
				_ = Parse(bytes.NewReader(body), isVMRemoteWrite, func([]prompb.TimeSeries, []prompb.MetricMetadata) error {
					return nil
				})
			}
		}
	})
}
