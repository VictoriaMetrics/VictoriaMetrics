package jaeger

import (
	"strings"
	"testing"

	"github.com/jaegertracing/jaeger/model"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

var span = &model.Span{
	TraceID:       model.TraceID{Low: 4300929031727684986, High: 12612784095082448122},
	SpanID:        0x04210a5512d46c4b,
	OperationName: "/",
	References:    []model.SpanRef{},
	Flags:         0,
	Duration:      198458,
	Tags: []model.KeyValue{
		{
			Key:   "otel.scope.name",
			VType: model.StringType,
			VStr:  "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
		},
		{
			Key:   "otel.scope.version",
			VType: model.StringType,
			VStr:  "0.60.0",
		},
		{
			Key:   "httpd.method",
			VType: model.StringType,
			VStr:  "GET",
		},
		{
			Key:   "httpd.scheme",
			VType: model.StringType,
			VStr:  "http",
		},
		{
			Key:   "net.host.name",
			VType: model.StringType,
			VStr:  "localhost",
		},
		{
			Key:    "net.host.port",
			VType:  model.Int64Type,
			VStr:   "",
			VInt64: 8080,
		},
		{
			Key:   "net.sock.peer",
			VType: model.StringType,
			VStr:  "127.0.0.1",
		},
		{
			Key:    "net.sock.peer.port",
			VType:  model.Int64Type,
			VStr:   "",
			VInt64: 59398,
		},
		{
			Key:   "user_agent.original",
			VType: model.StringType,
			VStr:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Safari/605.1.15",
		},
		{
			Key:   "http.target",
			VType: model.StringType,
			VStr:  "/",
		},
		{
			Key:   "net.protocol.version",
			VType: model.StringType,
			VStr:  "1.1",
		},
		{
			Key:   "http.route",
			VType: model.StringType,
			VStr:  "/",
		},
		{
			Key:    "http.status_code",
			VType:  model.Int64Type,
			VStr:   "",
			VInt64: 200,
		},
		{
			Key:   "span.kind",
			VType: model.StringType,
			VStr:  "server",
		},
	},

	Logs: nil,
	Process: &model.Process{
		ServiceName: "frontend",
		Tags: []model.KeyValue{
			{
				Key:   "host.name",
				VType: model.StringType,
				VStr:  "orbstack",
			},
			{
				Key:   "os.type",
				VType: model.StringType,
				VStr:  "linux",
			},
			{
				Key:   "telemetry.sdk.language",
				VType: model.StringType,
				VStr:  "go",
			},
			{
				Key:   "telemetry.sdk.name",
				VType: model.StringType,
				VStr:  "opentelemetry",
			},
			{
				Key:   "telemetry.sdk.version",
				VType: model.StringType,
				VStr:  "1.35.0",
			},
		},
	},
	ProcessID: "",
	Warnings:  nil,
}

func BenchmarkSpanToFields(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SpanToFields(span)
	}
}
func BenchmarkSpanToFieldsStringBuilder(b *testing.B) {
	sb := &strings.Builder{}
	sb.Grow(80)
	for i := 0; i < b.N; i++ {
		SpanToFieldsStringBuilder(sb, span)
	}
}

var bbufPool bytesutil.ByteBufferPool

func BenchmarkSpanToFieldsByteBuffer(b *testing.B) {
	bbuf := bbufPool.Get()
	defer bbufPool.Put(bbuf)
	for i := 0; i < b.N; i++ {
		SpanToFieldsByteBuffer(bbuf, span)
		bbuf.Reset()
	}
}
