package metricnamestats

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func BenchmarkTracker(b *testing.B) {
	b.ReportAllocs()
	mt := MustLoadFrom("testdata/"+b.Name(), 100_000_000)
	mt.getCurrentTs = func() uint64 {
		return 1
	}
	type testOp struct {
		t          byte
		metricName []byte
	}
	dataSet := []testOp{
		{'r', []byte("metric_1")},
		{'i', []byte("metric_2")},
		{'i', []byte("metric_3")},
		{'i', []byte("metric_3")},
		{'i', []byte("metric_4")},
		{'r', []byte("metric_3")},
		{'r', []byte("metric_3")},
		{'r', []byte("metric_3")},
	}
	b.ResetTimer()
	for range b.N {
		for _, op := range dataSet {
			switch op.t {
			case 'i':
				mt.RegisterIngestRequest(op.metricName)
			case 'r':
				mt.RegisterQueryRequest(op.metricName)
			}
		}
	}
	b.StopTimer()
	got := mt.GetStats(100, -1, "")
	got.sort()
	wantRecords := []StatRecord{
		{"metric_2", 0, 0},
		{"metric_4", 0, 0},
		{"metric_1", uint64(b.N), 1},
		{"metric_3", 3 * uint64(b.N), 1},
	}
	if !cmp.Equal(wantRecords, got.Records) {
		b.Fatalf("unexpected result: %s", cmp.Diff(wantRecords, got.Records))
	}
}
