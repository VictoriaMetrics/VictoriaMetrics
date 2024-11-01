package apptest

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// TestCase holds the state and defines clean-up procedure common for all test
// cases.
type TestCase struct {
	t   *testing.T
	cli *Client
}

// NewTestCase creates a new test case.
func NewTestCase(t *testing.T) *TestCase {
	return &TestCase{t, NewClient()}
}

// Dir returns the directory name that should be used by as the -storageDataDir.
func (tc *TestCase) Dir() string {
	return tc.t.Name()
}

// Client returns an instance of the client that can be used for interacting with
// the app(s) under test.
func (tc *TestCase) Client() *Client {
	return tc.cli
}

// Close performs the test case clean up, such as closing all client connections
// and removing the -storageDataDir directory.
//
// Note that the -storageDataDir is not removed in case of test case failure to
// allow for furher manual debugging.
func (tc *TestCase) Close() {
	tc.cli.CloseConnections()
	if !tc.t.Failed() {
		fs.MustRemoveAll(tc.Dir())
	}
}
