// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

// Package features provides an API for enabling features and checking if
// a given feature is enabled.
package features

import (
	"os"
	"strings"

	"github.com/DataDog/datadog-agent/pkg/trace/log"
)

// features keeps a map of all APM features as defined by the DD_APM_FEATURES
// environment variable at startup.
var features = map[string]struct{}{}

func init() {
	// Whoever imports this package, should have features readily available.
	Set(os.Getenv("DD_APM_FEATURES"))
}

// Set will set the given list of comma-separated features as active.
func Set(feats string) {
	for k := range features {
		delete(features, k)
	}
	all := strings.Split(feats, ",")
	for _, f := range all {
		features[strings.TrimSpace(f)] = struct{}{}
	}
	if active := All(); len(active) > 0 {
		log.Debugf("Loaded features: %v", active)
	}
}

// Has returns true if the feature f is present. Features are values
// of the DD_APM_FEATURES environment variable.
func Has(f string) bool {
	_, ok := features[f]
	return ok
}

// All returns a list of all the features configured by means of DD_APM_FEATURES.
func All() []string {
	var all []string
	for f := range features {
		all = append(all, f)
	}
	return all
}
