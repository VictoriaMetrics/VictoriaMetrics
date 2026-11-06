package stream

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogsketches"
	"github.com/VictoriaMetrics/easyproto"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	var m easyproto.Marshaler
	m.MessageMarshaler().AppendMessage(1).AppendString(1, "cpu")
	f.Add(m.Marshal(nil))
	f.Add([]byte{0x0a, 0xff})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		_ = Parse(bytes.NewReader(data), "", func([]*datadogsketches.Sketch) error { return nil })
	})
}

func FuzzStructuredSketches(f *testing.F) {
	f.Add([]byte("cpu"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var payload datadogsketches.SketchPayload
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&payload)
		payload.Sketches = append(payload.Sketches, &datadogsketches.Sketch{
			Metric: "cpu", Dogsketches: []*datadogsketches.Dogsketch{{Ts: 42, Cnt: 1}},
		})

		var m easyproto.Marshaler
		want := 0
		for _, sketch := range payload.Sketches {
			if sketch == nil {
				continue
			}
			want++
			s := m.MessageMarshaler().AppendMessage(1)
			s.AppendString(1, sketch.Metric)
			s.AppendString(2, sketch.Host)
			for _, tag := range sketch.Tags {
				s.AppendString(4, tag)
			}
			for _, dog := range sketch.Dogsketches {
				if dog == nil {
					continue
				}
				d := s.AppendMessage(7)
				d.AppendInt64(1, dog.Ts)
				d.AppendInt64(2, dog.Cnt)
				d.AppendDouble(3, dog.Min)
				d.AppendDouble(4, dog.Max)
				d.AppendDouble(6, dog.Sum)
				d.AppendSint32s(7, dog.K)
				d.AppendUint32s(8, dog.N)
			}
		}

		called := false
		err := Parse(bytes.NewReader(m.Marshal(nil)), "", func(sketches []*datadogsketches.Sketch) error {
			called = true
			if len(sketches) != want {
				t.Fatalf("got %d sketches; want %d", len(sketches), want)
			}
			return nil
		})
		if err != nil || !called {
			t.Fatalf("cannot parse structured sketches: %v; callback called: %v", err, called)
		}
	})
}
