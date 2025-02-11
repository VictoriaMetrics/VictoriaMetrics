package main

import (
	"context"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jaeger/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
)

func newSpanReaderPluginClient(t *testing.T) proto.SpanReaderPluginClient {
	conn, err := grpc.NewClient(fmt.Sprintf("0.0.0.0:%d", 17271), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("cannot connect to server: %v", err)
	}
	return proto.NewSpanReaderPluginClient(conn)
}

func newSpanWriterPluginClient(t *testing.T) proto.SpanWriterPluginClient {
	conn, err := grpc.NewClient(fmt.Sprintf("0.0.0.0:%d", 17271), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("cannot connect to server: %v", err)
	}
	return proto.NewSpanWriterPluginClient(conn)
}

func TestSpanWriter(t *testing.T) {
	// This is NOT a unit test. Please run the VictoriaLogs before executing this test.
	client := newSpanWriterPluginClient(t)
	req := &proto.WriteSpanRequest{}
	resp, err := client.WriteSpan(context.Background(), req)
	fmt.Println(resp, err)
}

func TestSpanReaderGetOperations(t *testing.T) {
	// This is NOT a unit test. Please run the VictoriaLogs before executing this test.
	client := newSpanReaderPluginClient(t)
	req := &proto.GetOperationsRequest{}
	resp, err := client.GetOperations(context.Background(), req)
	fmt.Println(resp, err)
}
