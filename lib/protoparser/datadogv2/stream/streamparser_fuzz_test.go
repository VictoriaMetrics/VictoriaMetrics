package stream

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/datadogv2"
	"github.com/VictoriaMetrics/easyproto"
	"sigs.k8s.io/randfill"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"series":[{"metric":"cpu","points":[{"timestamp":42,"value":1}]}]}`), false)
	var m easyproto.Marshaler
	m.MessageMarshaler().AppendMessage(1).AppendString(2, "cpu")
	f.Add(m.Marshal(nil), true)
	f.Add([]byte{0x0a, 0xff}, true)
	f.Fuzz(func(t *testing.T, data []byte, protobuf bool) {
		if len(data) > 4096 {
			t.Skip()
		}
		contentType := "application/json"
		if protobuf {
			contentType = "application/x-protobuf"
		}
		_ = Parse(bytes.NewReader(data), "", contentType, func([]datadogv2.Series) error { return nil })
	})
}

func FuzzStructuredRequest(f *testing.F) {
	f.Add([]byte(""), false)
	f.Add([]byte("series with points and resources"), true)

	f.Fuzz(func(t *testing.T, data []byte, protobuf bool) {
		if len(data) > 4096 {
			t.Skip()
		}

		var req datadogv2.Request
		randfill.NewFromGoFuzz(data).NilChance(0.2).MaxDepth(5).NumElements(0, 3).Fill(&req)
		req.Series = append(req.Series, datadogv2.Series{
			Metric: "cpu", Points: []datadogv2.Point{{Timestamp: 42, Value: 1}},
		})

		contentType := "application/json"
		var body []byte
		if protobuf {
			contentType = "application/x-protobuf"
			var m easyproto.Marshaler
			for _, item := range req.Series {
				s := m.MessageMarshaler().AppendMessage(1)
				s.AppendString(2, item.Metric)
				s.AppendString(7, item.SourceTypeName)
				for _, point := range item.Points {
					p := s.AppendMessage(4)
					p.AppendDouble(1, point.Value)
					p.AppendInt64(2, point.Timestamp)
				}
				for _, resource := range item.Resources {
					r := s.AppendMessage(1)
					r.AppendString(1, resource.Type)
					r.AppendString(2, resource.Name)
				}
				for _, tag := range item.Tags {
					s.AppendString(3, tag)
				}
			}
			body = m.Marshal(nil)
		} else {
			var err error
			body, err = json.Marshal(req)
			if err != nil {
				t.Skip()
			}
		}

		called := false
		err := Parse(bytes.NewReader(body), "", contentType, func(series []datadogv2.Series) error {
			called = true
			if len(series) != len(req.Series) {
				t.Fatalf("got %d series; want %d", len(series), len(req.Series))
			}
			return nil
		})
		if err != nil || !called {
			t.Fatalf("valid request: err=%v, callback called=%v", err, called)
		}
	})
}
