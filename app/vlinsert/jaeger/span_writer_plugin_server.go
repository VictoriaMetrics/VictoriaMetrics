package jaeger

import (
	"context"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jaeger"
	"github.com/jaegertracing/jaeger/model"
)

// A SpanWriterPluginServer represents plugin Jaeger interface to write gRPC storage backend
type SpanWriterPluginServer struct {
}

// WriteSpan writes spans
func (s *SpanWriterPluginServer) WriteSpan(ctx context.Context, span *model.Span) error {
	if span == nil {
		return fmt.Errorf("span not found")
	}

	cp, err := insertutils.GetJaegerCommonParams()
	if err != nil {
		return err
	}
	lmp := cp.NewLogMessageProcessor("jaeger")
	defer lmp.MustClose()

	fields, streamFields, err := jaeger.SpanToFields(span)
	lmp.AddRow(span.StartTime.UnixNano(), fields, streamFields)
	return nil
}

func (s *SpanWriterPluginServer) Close() error {
	return nil
}
