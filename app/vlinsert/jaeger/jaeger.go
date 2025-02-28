package jaeger

import (
	"fmt"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"log"
	"net"

	jaeger2 "github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/jaeger"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"

	"google.golang.org/grpc"
)

// MustInit - init for Jaeger gRPC storage backend
func MustInit() {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 17271))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	handler := shared.NewGRPCHandler(&shared.GRPCHandlerStorageImpl{
		SpanReader:          func() spanstore.Reader { return &jaeger2.SpanReaderPluginServer{} },
		SpanWriter:          func() spanstore.Writer { return &SpanWriterPluginServer{} },
		DependencyReader:    func() dependencystore.Reader { return &jaeger2.SpanReaderPluginServer{} },
		ArchiveSpanReader:   func() spanstore.Reader { return nil },
		ArchiveSpanWriter:   func() spanstore.Writer { return nil },
		StreamingSpanWriter: func() spanstore.Writer { return nil },
	})

	//proto.RegisterSpanWriterPluginServer(grpcServer, &SpanWriterPluginServer{})
	//proto.RegisterSpanReaderPluginServer(grpcServer, &jaeger2.SpanReaderPluginServer{})
	err = handler.Register(grpcServer)
	if err != nil {
		panic("unable to register Jaeger gRPC handler: " + err.Error())
	}

	go grpcServer.Serve(lis)
}

// MustStop - stop for Jaeger gRPC storage backend
func MustStop() {}
