package storage

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

func TestStorageMetadataRestart(t *testing.T) {
	path := t.TempDir()
	s := MustOpenStorage(path, OpenOptions{})
	rows := []metricsmetadata.Row{
		{MetricFamilyName: []byte("synthetic_requests_total"), Help: []byte("Synthetic requests"), Unit: []byte("requests"), Type: 1},
		{MetricFamilyName: []byte("synthetic_requests_total"), Help: []byte("Synthetic tenant requests"), Unit: []byte("requests"), Type: 1, AccountID: 7, ProjectID: 3},
	}
	s.AddMetadataRows(rows)
	check := func(t *testing.T) {
		t.Helper()
		got := s.GetMetadataRows(nil, 0, "synthetic_requests_total")
		if len(got) != len(rows) {
			t.Fatalf("unexpected metadata rows: got %d, want %d", len(got), len(rows))
		}
		for _, want := range rows {
			found := false
			for _, r := range got {
				if r.AccountID == want.AccountID && r.ProjectID == want.ProjectID &&
					bytes.Equal(r.MetricFamilyName, want.MetricFamilyName) && bytes.Equal(r.Help, want.Help) &&
					bytes.Equal(r.Unit, want.Unit) && r.Type == want.Type {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing metadata for tenant %d:%d", want.AccountID, want.ProjectID)
			}
		}
	}
	t.Run("before_restart", check)
	s.MustClose()
	s = MustOpenStorage(path, OpenOptions{})
	defer s.MustClose()
	t.Run("after_restart", check)
}
