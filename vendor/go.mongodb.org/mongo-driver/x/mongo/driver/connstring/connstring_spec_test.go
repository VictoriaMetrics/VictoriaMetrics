// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package connstring_test

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	testhelpers "go.mongodb.org/mongo-driver/internal/testutil/helpers"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

type host struct {
	Type string
	Host string
	Port json.Number
}

type auth struct {
	Username string
	Password *string
	DB       string
}

type testCase struct {
	Description string
	URI         string
	Valid       bool
	Warning     bool
	Hosts       []host
	Auth        *auth
	Options     map[string]interface{}
}

type testContainer struct {
	Tests []testCase
}

const connstringTestsDir = "../../../../data/connection-string/"
const urioptionsTestDir = "../../../../data/uri-options/"

func (h *host) toString() string {
	switch h.Type {
	case "unix":
		return h.Host
	case "ip_literal":
		if len(h.Port) == 0 {
			return "[" + h.Host + "]"
		} else {
			return "[" + h.Host + "]" + ":" + string(h.Port)
		}
	case "ipv4":
		fallthrough
	case "hostname":
		if len(h.Port) == 0 {
			return h.Host
		} else {
			return h.Host + ":" + string(h.Port)
		}
	}

	return ""
}

func hostsToStrings(hosts []host) []string {
	out := make([]string, len(hosts))

	for i, host := range hosts {
		out[i] = host.toString()
	}

	return out
}

func runTestsInFile(t *testing.T, dirname string, filename string, warningsError bool) {
	filepath := path.Join(dirname, filename)
	content, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)

	var container testContainer
	require.NoError(t, json.Unmarshal(content, &container))

	// Remove ".json" from filename.
	filename = filename[:len(filename)-5]

	for _, testCase := range container.Tests {
		runTest(t, filename, &testCase, warningsError)
	}
}

var skipTest = map[string]struct{}{
	"tlsAllowInvalidHostnames and tlsInsecure both present (and false) raises an error":    {},
	"tlsAllowInvalidHostnames and tlsInsecure both present (and true) raises an error":     {},
	"tlsInsecure and tlsAllowInvalidHostnames both present (and false) raises an error":    {},
	"tlsInsecure and tlsAllowInvalidHostnames both present (and true) raises an error":     {},
	"tlsAllowInvalidCertificates and tlsInsecure both present (and false) raises an error": {},
	"tlsAllowInvalidCertificates and tlsInsecure both present (and true) raises an error":  {},
	"tlsInsecure and tlsAllowInvalidCertificates both present (and false) raises an error": {},
	"tlsInsecure and tlsAllowInvalidCertificates both present (and true) raises an error":  {},
	"Invalid tlsAllowInvalidHostnames causes a warning":                                    {},
	"tlsAllowInvalidHostnames is parsed correctly":                                         {},
	"Invalid tlsAllowInvalidCertificates causes a warning":                                 {},
	"tlsAllowInvalidCertificates is parsed correctly":                                      {},
	"Invalid serverSelectionTryOnce causes a warning":                                      {},
	"Valid options specific to single-threaded drivers are parsed correctly":               {},
}

func runTest(t *testing.T, filename string, test *testCase, warningsError bool) {
	t.Run(test.Description, func(t *testing.T) {
		if _, skip := skipTest[test.Description]; skip {
			t.Skip()
		}
		cs, err := connstring.Parse(test.URI)
		// Since we don't have warnings in Go, we return warnings as errors.
		//
		// This is a bit unfortuante, but since we do raise warnings as errors with the newer
		// URI options, but don't with some of the older things, we do a switch on the filename
		// here. We are trying to not break existing user applications that have unrecognized
		// options.
		if test.Valid && !(test.Warning && warningsError) {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			return
		}

		require.Equal(t, test.URI, cs.Original)

		if test.Hosts != nil {
			require.Equal(t, hostsToStrings(test.Hosts), cs.Hosts)
		}

		if test.Auth != nil {
			require.Equal(t, test.Auth.Username, cs.Username)

			if test.Auth.Password == nil {
				require.False(t, cs.PasswordSet)
			} else {
				require.True(t, cs.PasswordSet)
				require.Equal(t, *test.Auth.Password, cs.Password)
			}

			if test.Auth.DB != cs.Database {
				require.Equal(t, test.Auth.DB, cs.AuthSource)
			} else {
				require.Equal(t, test.Auth.DB, cs.Database)
			}
		}

		// Check that all options are present.
		testhelpers.VerifyConnStringOptions(t, cs, test.Options)

		// Check that non-present options are unset. This will be redundant with the above checks
		// for options that are present.
		var ok bool

		_, ok = test.Options["maxpoolsize"]
		require.Equal(t, ok, cs.MaxPoolSizeSet)
	})
}

// Test case for all connection string spec tests.
func TestConnStringSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, connstringTestsDir) {
		runTestsInFile(t, connstringTestsDir, file, false)
	}
}

func TestURIOptionsSpec(t *testing.T) {
	for _, file := range testhelpers.FindJSONFilesInDir(t, urioptionsTestDir) {
		runTestsInFile(t, urioptionsTestDir, file, true)
	}
}
