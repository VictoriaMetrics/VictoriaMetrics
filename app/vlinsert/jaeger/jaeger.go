package jaeger

import (
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jaeger/proto"
)

func MustInit() {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 17271))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	proto.RegisterSpanWriterPluginServer(grpcServer, &SpanWriterPluginServer{})
	proto.RegisterSpanReaderPluginServer(grpcServer, &SpanReaderPluginServer{})

	go grpcServer.Serve(lis)
}

func MustStop() {}
