// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package traceutil

import (
	"bytes"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/tinylib/msgp/msgp"
)

const (
	// This is a special metric, it's 1 if the span is top-level, 0 if not.
	topLevelKey = "_top_level"

	// measuredKey is a special metric flag that marks a span for trace metrics calculation.
	measuredKey = "_dd.measured"
	// tracerTopLevelKey is a metric flag set by tracers on top_level spans
	tracerTopLevelKey = "_dd.top_level"
	// partialVersionKey is a metric carrying the snapshot seq number in the case the span is a partial snapshot
	partialVersionKey = "_dd.partial_version"
)

// HasTopLevel returns true if span is top-level.
func HasTopLevel(s *pb.Span) bool {
	return s.Metrics[topLevelKey] == 1
}

// UpdateTracerTopLevel sets _top_level tag on spans flagged by the tracer
func UpdateTracerTopLevel(s *pb.Span) {
	if s.Metrics[tracerTopLevelKey] == 1 {
		SetMetric(s, topLevelKey, 1)
	}
}

// IsMeasured returns true if a span should be measured (i.e., it should get trace metrics calculated).
func IsMeasured(s *pb.Span) bool {
	return s.Metrics[measuredKey] == 1
}

// IsPartialSnapshot returns true if the span is a partial snapshot.
// This kind of spans are partial images of long-running spans.
// When incomplete, a partial snapshot has a metric _dd.partial_version which is a positive integer.
// The metric usually increases each time a new version of the same span is sent by the tracer
func IsPartialSnapshot(s *pb.Span) bool {
	v, ok := s.Metrics[partialVersionKey]
	return ok && v >= 0
}

// SetTopLevel sets the top-level attribute of the span.
func SetTopLevel(s *pb.Span, topLevel bool) {
	if !topLevel {
		if s.Metrics == nil {
			return
		}
		delete(s.Metrics, topLevelKey)
		return
	}
	// Setting the metrics value, so that code downstream in the pipeline
	// can identify this as top-level without recomputing everything.
	SetMetric(s, topLevelKey, 1)
}

// SetMetric sets the metric at key to the val on the span s.
func SetMetric(s *pb.Span, key string, val float64) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]float64)
	}
	s.Metrics[key] = val
}

// SetMeta sets the metadata at key to the val on the span s.
func SetMeta(s *pb.Span, key, val string) {
	if s.Meta == nil {
		s.Meta = make(map[string]string)
	}
	s.Meta[key] = val
}

// GetMeta gets the metadata value in the span Meta map.
func GetMeta(s *pb.Span, key string) (string, bool) {
	if s.Meta == nil {
		return "", false
	}
	val, ok := s.Meta[key]
	return val, ok
}

// GetMetaDefault gets the metadata value in the span Meta map and fallbacks to fallback.
func GetMetaDefault(s *pb.Span, key, fallback string) string {
	if s.Meta == nil {
		return fallback
	}
	if val, ok := s.Meta[key]; ok {
		return val
	}
	return fallback
}

// SetMetaStruct sets the structured metadata at key to the val on the span s.
func SetMetaStruct(s *pb.Span, key string, val interface{}) error {
	var b bytes.Buffer

	if s.MetaStruct == nil {
		s.MetaStruct = make(map[string][]byte)
	}
	writer := msgp.NewWriter(&b)
	err := writer.WriteIntf(val)
	if err != nil {
		return err
	}
	writer.Flush()
	s.MetaStruct[key] = b.Bytes()
	return nil
}

// GetMetaStruct gets the structured metadata value in the span MetaStruct map.
func GetMetaStruct(s *pb.Span, key string) (interface{}, bool) {
	if s.MetaStruct == nil {
		return nil, false
	}
	if rawVal, ok := s.MetaStruct[key]; ok {
		val, _, err := msgp.ReadIntfBytes(rawVal)
		if err != nil {
			ok = false
		}
		return val, ok
	}
	return nil, false
}
