package datadogv2

import (
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/easyproto"
	"sigs.k8s.io/randfill"
)

func FuzzUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`{"series":[{"metric":"cpu","points":[{"timestamp":42,"value":1}]}]}`))
	f.Add([]byte(`{"series":[{"metric":"cpu","points":[{"timestamp":0,"value":1}],"resources":[{"type":"host","name":"vm"}],"tags":["env:test"]}]}`))
	f.Add([]byte(`{"series":[{"points":`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var req Request
		_ = UnmarshalJSON(&req, data)

		var generated Request
		randfill.NewFromGoFuzz(data).NumElements(0, 3).Fill(&generated)
		if b, err := json.Marshal(generated); err == nil {
			_ = UnmarshalJSON(&req, b)
		}
	})
}

func FuzzUnmarshalProtobuf(f *testing.F) {
	var m easyproto.Marshaler
	series := m.MessageMarshaler().AppendMessage(1)
	series.AppendString(2, "cpu")
	series.AppendMessage(4)
	f.Add(m.Marshal(nil))

	// Series with metric, point, resource and tag.
	m.Reset()
	series = m.MessageMarshaler().AppendMessage(1)
	series.AppendString(2, "cpu")
	point := series.AppendMessage(4)
	point.AppendDouble(1, 1)
	point.AppendInt64(2, 42)
	resource := series.AppendMessage(1)
	resource.AppendString(1, "host")
	resource.AppendString(2, "vm")
	series.AppendString(3, "env")
	f.Add(m.Marshal(nil))

	// Truncated length-delimited Series.
	f.Add([]byte{0x0a, 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var req Request
		_ = UnmarshalProtobuf(&req, data)

		var metric string
		randfill.NewFromGoFuzz(data).Fill(&metric)
		var m easyproto.Marshaler
		series := m.MessageMarshaler().AppendMessage(1)
		series.AppendString(2, metric)
		point := series.AppendMessage(4)
		point.AppendDouble(1, 1)
		point.AppendInt64(2, 42)
		_ = UnmarshalProtobuf(&req, m.Marshal(nil))
	})
}
