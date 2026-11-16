package datadogsketches

import (
	"testing"

	"github.com/VictoriaMetrics/easyproto"
	"sigs.k8s.io/randfill"
)

func FuzzSketchPayloadUnmarshal(f *testing.F) {
	var m easyproto.Marshaler
	m.MessageMarshaler().AppendMessage(1).AppendString(1, "cpu")
	f.Add(m.Marshal(nil))

	m.Reset()
	sketch := m.MessageMarshaler().AppendMessage(1)
	sketch.AppendString(1, "cpu")
	sketch.AppendString(2, "vm")
	sketch.AppendString(4, "env")
	dog := sketch.AppendMessage(7)
	dog.AppendInt64(1, 42)
	dog.AppendInt64(2, 2)
	dog.AppendDouble(3, 1)
	dog.AppendDouble(4, 1)
	dog.AppendDouble(6, 1)
	dog.AppendSint32s(7, []int32{1})
	dog.AppendUint32s(8, []uint32{2})
	f.Add(m.Marshal(nil))

	// Truncated length-delimited Sketch.
	f.Add([]byte{0x0a, 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var payload SketchPayload
		if payload.UnmarshalProtobuf(data) == nil {
			for _, sketch := range payload.Sketches {
				_ = sketch.RowsCount()
				_ = sketch.ToSummary()
			}
		}

		var metric string
		randfill.NewFromGoFuzz(data).Fill(&metric)
		var m easyproto.Marshaler
		m.MessageMarshaler().AppendMessage(1).AppendString(1, metric)
		if payload.UnmarshalProtobuf(m.Marshal(nil)) == nil {
			for _, sketch := range payload.Sketches {
				_ = sketch.RowsCount()
				_ = sketch.ToSummary()
			}
		}
	})
}
