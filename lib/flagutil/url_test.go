package flagutil

import (
	"testing"
)

func TestIsURLFlag(t *testing.T) {
	testCases := []struct {
		desc     string
		flagName string
		expected bool
		register bool
	}{
		{
			desc:     "contains url",
			flagName: "remoteWrite.url",
			expected: true,
			register: false,
		},
		{
			desc:     "does not contain url, registered",
			flagName: "remoteWrite.target",
			expected: true,
			register: true,
		},
		{
			desc:     "does not contain url, unregistered",
			flagName: "remoteWrite.foo",
			expected: false,
			register: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if tC.register {
				RegisterURLFlag(tC.flagName)
			}
			result := IsURLFlag(tC.flagName)
			if tC.expected != result {
				t.Errorf("unexpected result; got %t; want %t", result, tC.expected)
			}
		})
	}
}

func TestRedactURLFlagPassword(t *testing.T) {
	testCases := []struct {
		desc      string
		flagValue string
		expected  string
	}{
		{
			desc:      "valid url, no user:pass",
			flagValue: "http://hostname:1234/path",
			expected:  "http://hostname:1234/path",
		},
		{
			desc:      "valid url, user:pass",
			flagValue: "http://foo:bar@hostname:1234/path",
			expected:  "http://foo:REDACTED@hostname:1234/path",
		},
		{
			desc:      "valid url, user, no pass",
			flagValue: "http://foo:@hostname:1234/path",
			expected:  "http://foo:@hostname:1234/path",
		},
		{
			desc:      "invalid url, user, no pass",
			flagValue: "not-a-url",
			expected:  "not-a-url",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			result := RedactURLFlagPassword(tC.flagValue)
			if tC.expected != result {
				t.Errorf("unexpected result; got %s; want %s", result, tC.expected)
			}
		})
	}
}
