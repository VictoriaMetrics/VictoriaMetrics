package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type sparseTestObject struct {
	name    string
	matched bool
}

func (o *sparseTestObject) key() string { return o.name }

func (o *sparseTestObject) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	labels := promutil.GetLabels()
	labels.Add("name", o.name)
	if o.matched {
		labels.Add("matched", "yes")
	}
	return []*promutil.Labels{labels}
}

func newSparseTestAPIWatcher(accept func(*promutil.Labels) bool) *apiWatcher {
	return &apiWatcher{
		swcFunc: func(labels *promutil.Labels) any {
			if !accept(labels) {
				// Exercise the typed-nil path in getScrapeWorkObjectsForLabels.
				return (*string)(nil)
			}
			name := labels.Get("name")
			return &name
		},
		swosByURLWatcher: make(map[*urlWatcher]map[string][]any),
		swosCount:        &metrics.Counter{},
	}
}

func checkSparseTestState(t *testing.T, aw *apiWatcher, uw *urlWatcher, want map[string][]string) {
	t.Helper()
	got := make(map[string][]string)
	for key, swos := range aw.swosByURLWatcher[uw] {
		if len(swos) == 0 {
			t.Fatalf("empty result retained for key %q", key)
		}
		for _, swo := range swos {
			got[key] = append(got[key], *swo.(*string))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected scrape works; got %#v; want %#v", got, want)
	}
	count := 0
	for _, swos := range want {
		count += len(swos)
	}
	if gotCount := aw.swosCount.Get(); gotCount != uint64(count) {
		t.Fatalf("unexpected scrape work count; got %d; want %d", gotCount, count)
	}
}

func TestSparseFullRebuild(t *testing.T) {
	uw := &urlWatcher{gw: &groupWatcher{}}
	aws := make(map[*apiWatcher]struct{})
	watchers := make(map[*apiWatcher]string)
	for _, name := range []string{"A", "B"} {
		aw := newSparseTestAPIWatcher(func(labels *promutil.Labels) bool {
			return labels.Get("matched") == "yes" && labels.Get("name") == name
		})
		aws[aw] = struct{}{}
		watchers[aw] = name
	}
	uw.recreateScrapeWorksLocked(map[string]object{
		"A": &sparseTestObject{name: "A", matched: true},
		"B": &sparseTestObject{name: "B", matched: true},
		"C": &sparseTestObject{name: "C"},
	}, aws)
	for aw, name := range watchers {
		checkSparseTestState(t, aw, uw, map[string][]string{name: {name}})
	}

	// A full rebuild with only empty results must replace the old state.
	uw.recreateScrapeWorksLocked(map[string]object{
		"A": &sparseTestObject{name: "A"},
		"B": &sparseTestObject{name: "B"},
	}, aws)
	for aw := range watchers {
		checkSparseTestState(t, aw, uw, map[string][]string{})
	}
}

func TestSparseIncrementalUpdates(t *testing.T) {
	aw := newSparseTestAPIWatcher(func(labels *promutil.Labels) bool { return labels.Get("matched") == "yes" })
	aws := map[*apiWatcher]struct{}{aw: {}}
	uw := &urlWatcher{gw: &groupWatcher{}, aws: aws}
	uw.recreateScrapeWorksLocked(map[string]object{
		"A": &sparseTestObject{name: "A", matched: true},
		"B": &sparseTestObject{name: "B", matched: true},
	}, aws)
	checkSparseTestState(t, aw, uw, map[string][]string{"A": {"A"}, "B": {"B"}})

	// An empty result deletes A; B is outside the delta and must survive.
	uw.updateScrapeWorksLocked(map[string]object{"A": &sparseTestObject{name: "A"}}, aws)
	checkSparseTestState(t, aw, uw, map[string][]string{"B": {"B"}})
	uw.updateScrapeWorksLocked(map[string]object{"A": &sparseTestObject{name: "A", matched: true}}, aws)
	checkSparseTestState(t, aw, uw, map[string][]string{"A": {"A"}, "B": {"B"}})
}
