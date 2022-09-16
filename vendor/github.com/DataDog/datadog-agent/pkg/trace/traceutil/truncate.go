// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package traceutil

import (
	"github.com/DataDog/datadog-agent/pkg/trace/config/features"
)

// MaxResourceLen the maximum length the resource can have
var MaxResourceLen = 5000

func init() {
	if features.Has("big_resource") {
		MaxResourceLen = 15000
	}
}

const (
	// MaxMetaKeyLen the maximum length of metadata key
	MaxMetaKeyLen = 200
	// MaxMetaValLen the maximum length of metadata value
	MaxMetaValLen = 25000
	// MaxMetricsKeyLen the maximum length of a metric name key
	MaxMetricsKeyLen = MaxMetaKeyLen
)

// TruncateResource truncates a span's resource to the maximum allowed length.
// It returns true if the input was below the max size.
func TruncateResource(r string) (string, bool) {
	return TruncateUTF8(r, MaxResourceLen), len(r) <= MaxResourceLen
}

// TruncateUTF8 truncates the given string to make sure it uses less than limit bytes.
// If the last character is an utf8 character that would be splitten, it removes it
// entirely to make sure the resulting string is not broken.
func TruncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	var lastValidIndex int
	for i := range s {
		if i > limit {
			return s[:lastValidIndex]
		}
		lastValidIndex = i
	}
	return s
}
