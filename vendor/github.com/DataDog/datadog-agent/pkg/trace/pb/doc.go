// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package pb contains the data structures used by the trace agent to communicate
// with tracers and the Datadog API. Note that the "//go:generate" directives from this
// package were removed because the generated files were manually edited to create
// adaptions (see decoder.go).
//
// TODO: eventually move this to https://github.com/DataDog/agent-payload/v5
package pb
