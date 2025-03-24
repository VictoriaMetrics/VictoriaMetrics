package jaeger

import (
	"context"
	"fmt"

	"github.com/jaegertracing/jaeger/model"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jaeger"
)

// A SpanWriterPluginServer represents plugin Jaeger interface to write gRPC storage backend
type SpanWriterPluginServer struct {
}

var bbufPool bytesutil.ByteBufferPool

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
	// bytes buf here
	bbuf := bbufPool.Get()
	defer bbufPool.Put(bbuf)
	bbuf.Reset()
	fields, streamFields, err := jaeger.SpanToFields(span)
	lmp.AddRow(span.StartTime.UnixNano(), fields, streamFields)
	// bytes bf clear
	return nil
}

func (s *SpanWriterPluginServer) Close() error {
	return nil
}
