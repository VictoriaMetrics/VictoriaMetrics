package fscommon

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/appmetrics"
)

func TestAppendFilesSkipsUncleanShutdownMarker(t *testing.T) {
	dir := t.TempDir()
	regularPath := filepath.Join(dir, "regular.bin")
	markerPath := filepath.Join(dir, appmetrics.UncleanShutdownMarkerFilename)
	if err := os.WriteFile(regularPath, []byte("data"), 0600); err != nil {
		t.Fatalf("cannot create regular file: %s", err)
	}
	if err := os.WriteFile(markerPath, nil, 0600); err != nil {
		t.Fatalf("cannot create unclean shutdown marker: %s", err)
	}

	files, err := AppendFiles(nil, dir)
	if err != nil {
		t.Fatalf("unexpected error from AppendFiles: %s", err)
	}
	if !slices.Contains(files, regularPath) {
		t.Fatalf("expected %q in AppendFiles result; got %q", regularPath, files)
	}
	if slices.Contains(files, markerPath) {
		t.Fatalf("AppendFiles must skip unclean shutdown marker %q; got %q", markerPath, files)
	}
}
