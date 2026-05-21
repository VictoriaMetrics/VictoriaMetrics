package ecsmetadata

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestWriteMetrics(t *testing.T) {
	f := func(taskPath, statsPath, snapshotPath string) {
		t.Helper()
		resp := loadFixture(t, taskPath, statsPath)

		var buf bytes.Buffer
		resp.WriteMetrics(&buf)

		want, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatalf("cannot read snapshot %s: %s", snapshotPath, err)
		}
		got := sortLines(buf.String())
		exp := sortLines(string(want))
		if got != exp {
			t.Fatalf("unexpected WriteMetrics result;\ngot\n%s\nwant\n%s", got, exp)
		}
	}

	f("testdata/fixtures/task.json", "testdata/fixtures/task_stats.json", "testdata/snapshots/metrics.txt")
}

func sortLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func loadFixture(t *testing.T, taskPath, statsPath string) *TaskResponse {
	t.Helper()

	taskData, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	statsData, err := os.ReadFile(statsPath)
	if err != nil {
		t.Fatal(err)
	}

	var meta TaskMetadata
	if err = json.Unmarshal(taskData, &meta); err != nil {
		t.Fatalf("%s: %s", taskPath, err)
	}
	var stats map[string]*ContainerStats
	if err = json.Unmarshal(statsData, &stats); err != nil {
		t.Fatalf("%s: %s", statsPath, err)
	}

	return &TaskResponse{Metadata: meta, Stats: stats}
}