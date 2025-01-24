package jaeger

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jaeger/proto"
	"google.golang.org/grpc"
	"log"
	"net"
)

func MustInit() {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", 17271))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	proto.RegisterSpanWriterPluginServer(grpcServer, &SpanWriterPluginServer{})
	proto.RegisterSpanReaderPluginServer(grpcServer, &proto.UnimplementedSpanReaderPluginServer{})

	go grpcServer.Serve(lis)
}

func MustStop() {}
