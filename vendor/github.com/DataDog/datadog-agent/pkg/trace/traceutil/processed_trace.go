// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package traceutil

import (
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
)

// ProcessedTrace represents a trace being processed in the agent.
type ProcessedTrace struct {
	TraceChunk             *pb.TraceChunk
	Root                   *pb.Span
	TracerEnv              string
	AppVersion             string
	TracerHostname         string
	ClientDroppedP0sWeight float64
}
