package migrationstate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

type testParams struct {
	SrcAddr        string   `json:"srcAddr"`
	DstExtraLabels []string `json:"dstExtraLabels"`
}

func TestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.state")
	params := testParams{SrcAddr: "http://src", DstExtraLabels: []string{"b=2", "a=1"}}

	// Load creates the missing state file.
	st, err := Load(path, params)
	if err != nil {
		t.Fatalf("Load() error: %s", err)
	}
	if st.DoneCount() != 0 {
		t.Fatalf("expecting no done units for the new state; got %d", st.DoneCount())
	}
	u1 := &Unit{MetricName: "foo bar", TR: storage.TimeRange{MinTimestamp: 1, MaxTimestamp: 2}}
	u2 := &Unit{Tenant: "1:0", MetricName: "foo", TR: storage.TimeRange{MinTimestamp: 1, MaxTimestamp: 2}}
	u3 := &Unit{MetricName: "baz", TR: storage.TimeRange{MinTimestamp: 3, MaxTimestamp: 4}}
	if err := st.MarkDone(u1); err != nil {
		t.Fatalf("MarkDone() error: %s", err)
	}
	if err := st.MarkDone(u2); err != nil {
		t.Fatalf("MarkDone() error: %s", err)
	}
	st.Close()

	// The first line is params in JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read state file: %s", err)
	}
	want := `{"srcAddr":"http://src","dstExtraLabels":["b=2","a=1"]}` + "\n" + u1.StateLine() + "\n" + u2.StateLine() + "\n"
	if string(data) != want {
		t.Fatalf("unexpected state file contents\ngot\n%s\nwant\n%s", data, want)
	}

	// Simulate a partially written line for "foo_suffix". It parses as a line for "foo",
	// but it must be ignored, since it isn't newline-terminated.
	u4 := &Unit{MetricName: "foo", TR: storage.TimeRange{MinTimestamp: 3, MaxTimestamp: 4}}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("cannot open state file: %s", err)
	}
	if _, err := f.WriteString(u4.StateLine()); err != nil {
		t.Fatalf("cannot write to state file: %s", err)
	}
	_ = f.Close()

	st, err = Load(path, params)
	if err != nil {
		t.Fatalf("Load() error: %s", err)
	}
	if !st.IsDone(u1) || !st.IsDone(u2) || st.IsDone(u3) || st.IsDone(u4) {
		t.Fatalf("unexpected done units: %v", st.done)
	}

	// The incomplete last line is removed by Load, so the appended line isn't glued to it.
	if err := st.MarkDone(u3); err != nil {
		t.Fatalf("MarkDone() error: %s", err)
	}
	st.Close()
	st, err = Load(path, params)
	if err != nil {
		t.Fatalf("Load() error: %s", err)
	}
	if !st.IsDone(u1) || !st.IsDone(u2) || !st.IsDone(u3) || st.IsDone(u4) || st.DoneCount() != 3 {
		t.Fatalf("unexpected done units after append: %v", st.done)
	}
	st.Close()

	// The state file with different params is rejected.
	p := params
	p.SrcAddr = "http://another-src"
	if _, err := Load(path, p); err == nil {
		t.Fatalf("expecting error for different params")
	}

	// The state file without complete header is rejected.
	if err := os.WriteFile(path, []byte(`{"srcAddr":`), 0o644); err != nil {
		t.Fatalf("cannot write state file: %s", err)
	}
	if _, err := Load(path, params); err == nil {
		t.Fatalf("expecting error for incomplete header")
	}
}
