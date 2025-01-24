package jaeger

import (
	"context"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jaeger/proto"
)

type SpanReaderPluginServer struct{}

func (s *SpanReaderPluginServer) GetTrace(*proto.GetTraceRequest, proto.SpanReaderPlugin_GetTraceServer) error {
	return nil
}

func (s *SpanReaderPluginServer) GetServices(context.Context, *proto.GetServicesRequest) (*proto.GetServicesResponse, error) {
	return nil, nil
}

func (s *SpanReaderPluginServer) GetOperations(context.Context, *proto.GetOperationsRequest) (*proto.GetOperationsResponse, error) {
	return nil, nil
}

func (s *SpanReaderPluginServer) FindTraces(*proto.FindTracesRequest, proto.SpanReaderPlugin_FindTracesServer) error {
	return nil
}

func (s *SpanReaderPluginServer) FindTraceIDs(context.Context, *proto.FindTraceIDsRequest) (*proto.FindTraceIDsResponse, error) {
	return nil, nil
}
