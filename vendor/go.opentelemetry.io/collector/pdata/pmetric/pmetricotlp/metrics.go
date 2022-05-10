// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pmetricotlp // import "go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

import (
	"bytes"
	"context"

	"github.com/gogo/protobuf/jsonpb"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/pdata/internal"
	otlpcollectormetrics "go.opentelemetry.io/collector/pdata/internal/data/protogen/collector/metrics/v1"
	"go.opentelemetry.io/collector/pdata/internal/otlp"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var jsonMarshaler = &jsonpb.Marshaler{}
var jsonUnmarshaler = &jsonpb.Unmarshaler{}

// Response represents the response for gRPC/HTTP client/server.
type Response struct {
	orig *otlpcollectormetrics.ExportMetricsServiceResponse
}

// NewResponse returns an empty Response.
func NewResponse() Response {
	return Response{orig: &otlpcollectormetrics.ExportMetricsServiceResponse{}}
}

// MarshalProto marshals Response into proto bytes.
func (mr Response) MarshalProto() ([]byte, error) {
	return mr.orig.Marshal()
}

// UnmarshalProto unmarshalls Response from proto bytes.
func (mr Response) UnmarshalProto(data []byte) error {
	return mr.orig.Unmarshal(data)
}

// MarshalJSON marshals Response into JSON bytes.
func (mr Response) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := jsonMarshaler.Marshal(&buf, mr.orig); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalJSON unmarshalls Response from JSON bytes.
func (mr Response) UnmarshalJSON(data []byte) error {
	return jsonUnmarshaler.Unmarshal(bytes.NewReader(data), mr.orig)
}

// Request represents the request for gRPC/HTTP client/server.
// It's a wrapper for pmetric.Metrics data.
type Request struct {
	orig *otlpcollectormetrics.ExportMetricsServiceRequest
}

// NewRequest returns an empty Request.
func NewRequest() Request {
	return Request{orig: &otlpcollectormetrics.ExportMetricsServiceRequest{}}
}

// NewRequestFromMetrics returns a Request from pmetric.Metrics.
// Because Request is a wrapper for pmetric.Metrics,
// any changes to the provided Metrics struct will be reflected in the Request and vice versa.
func NewRequestFromMetrics(m pmetric.Metrics) Request {
	return Request{orig: internal.MetricsToOtlp(m)}
}

// MarshalProto marshals Request into proto bytes.
func (mr Request) MarshalProto() ([]byte, error) {
	return mr.orig.Marshal()
}

// UnmarshalProto unmarshalls Request from proto bytes.
func (mr Request) UnmarshalProto(data []byte) error {
	return mr.orig.Unmarshal(data)
}

// MarshalJSON marshals Request into JSON bytes.
func (mr Request) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	if err := jsonMarshaler.Marshal(&buf, mr.orig); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalJSON unmarshalls Request from JSON bytes.
func (mr Request) UnmarshalJSON(data []byte) error {
	if err := jsonUnmarshaler.Unmarshal(bytes.NewReader(data), mr.orig); err != nil {
		return err
	}
	otlp.InstrumentationLibraryMetricsToScope(mr.orig.ResourceMetrics)
	return nil
}

// Deprecated: [v0.50.0] Use NewRequestFromMetrics instead.
func (mr Request) SetMetrics(ld pmetric.Metrics) {
	*mr.orig = *internal.MetricsToOtlp(ld)
}

func (mr Request) Metrics() pmetric.Metrics {
	return internal.MetricsFromOtlp(mr.orig)
}

// Client is the client API for OTLP-GRPC Metrics service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type Client interface {
	// Export pmetric.Metrics to the server.
	//
	// For performance reasons, it is recommended to keep this RPC
	// alive for the entire life of the application.
	Export(ctx context.Context, request Request, opts ...grpc.CallOption) (Response, error)
}

type metricsClient struct {
	rawClient otlpcollectormetrics.MetricsServiceClient
}

// NewClient returns a new Client connected using the given connection.
func NewClient(cc *grpc.ClientConn) Client {
	return &metricsClient{rawClient: otlpcollectormetrics.NewMetricsServiceClient(cc)}
}

func (c *metricsClient) Export(ctx context.Context, request Request, opts ...grpc.CallOption) (Response, error) {
	rsp, err := c.rawClient.Export(ctx, request.orig, opts...)
	return Response{orig: rsp}, err
}

// Server is the server API for OTLP gRPC MetricsService service.
type Server interface {
	// Export is called every time a new request is received.
	//
	// For performance reasons, it is recommended to keep this RPC
	// alive for the entire life of the application.
	Export(context.Context, Request) (Response, error)
}

// RegisterServer registers the Server to the grpc.Server.
func RegisterServer(s *grpc.Server, srv Server) {
	otlpcollectormetrics.RegisterMetricsServiceServer(s, &rawMetricsServer{srv: srv})
}

type rawMetricsServer struct {
	srv Server
}

func (s rawMetricsServer) Export(ctx context.Context, request *otlpcollectormetrics.ExportMetricsServiceRequest) (*otlpcollectormetrics.ExportMetricsServiceResponse, error) {
	otlp.InstrumentationLibraryMetricsToScope(request.ResourceMetrics)
	rsp, err := s.srv.Export(ctx, Request{orig: request})
	return rsp.orig, err
}
