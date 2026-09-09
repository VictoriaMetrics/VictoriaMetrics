package appmetrics

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUncleanShutdownLifecycle(t *testing.T) {

	t.Cleanup(func() {
		uncleanShutdownEnabled.Store(false)
	})
	dirPath := t.TempDir()
	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)

	// unclean logic is disabled. the unclean shutdown metric should not be exposed
	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	if strings.Contains(bb.String(), "vm_app_prev_shutdown_unclean") {
		t.Fatalf("unexpected unclean shutdown metric before starting the marker")
	}

	// clean start, the metric must report 0
	MustCreateUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 0)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("cannot stat the running marker after the first start: %s", err)
	}
	MustRemoveUncleanShutdownMarker(dirPath)
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}
	uncleanShutdownEnabled.Store(false)

	// simulate prev unclean shutdown, the metric must report 1
	if err := os.WriteFile(markerPath, nil, 0600); err != nil {
		t.Fatalf("cannot create test marker: %s", err)
	}
	MustCreateUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 1)
	MustRemoveUncleanShutdownMarker(dirPath)
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}
}

func mustContainUncleanShutdownMetric(t *testing.T, value uint64) {
	t.Helper()

	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	want := "vm_app_prev_shutdown_unclean " + strconv.FormatUint(value, 10) + "\n"
	if !strings.Contains(bb.String(), want) {
		t.Fatalf("missing %q in the exported app metrics", want)
	}
}
