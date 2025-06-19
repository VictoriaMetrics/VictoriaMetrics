package storage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
)

func TestSearchContext(t *testing.T) {
	sc := getSearchContext(noDeadline, "test", querytracer.New(true, "test"))

	sc2 := sc.NewChild("child")
	sc2.Printf("test %d", 123)
	sc2.Done()

	sc.Done()

	s := sc.ToJSON()
	if s == "" {
		t.Fatalf("unexpected empty JSON trace")
	}

	sc = getSearchContext(noDeadline, "test2", nil)
	sc.Printf("test %d", 456)
	sc.Done()
	if sc.ToJSON() != "" {
		t.Fatalf("unexpected non-empty JSON trace for nil Tracer")
	}
}

func TestPropagatesReadMetricsToParent(t *testing.T) {
	sc := getSearchContext(noDeadline, "test2", querytracer.New(true, "test2"))
	sc2 := sc.NewChild("child2")
	sc2.trackReadMetricIDs(40)
	sc3 := sc2.NewChild("child3")
	sc3.trackReadMetricIDs(50)
	sc3.Done()
	sc2.Done()
	sc.Done()
	if sc.readMetricIDs.Load() != 90 {
		t.Fatalf("unexpected readMetricIDs value %d; want 120", sc.readMetricIDs.Load())
	}
}
