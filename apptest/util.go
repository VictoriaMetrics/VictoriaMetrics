package apptest

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// testRemoveAll removes all data that has been created by a test case if the
// test case has passed.
func testRemoveAll(t *testing.T) {
	if !t.Failed() {
		fs.MustRemoveAll(t.Name())
	}
}
