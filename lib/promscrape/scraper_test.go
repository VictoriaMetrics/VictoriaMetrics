package promscrape

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestScraperReload(t *testing.T) {
	f := func(oldCfgData, newCfgData string, reloadExpected bool) {
		pushData := func(_ *auth.Token, _ *prompbmarshal.WriteRequest) {}
		globalStopChan = make(chan struct{})
		defer close(globalStopChan)

		randName := rand.Int()
		sg := newScraperGroup(fmt.Sprintf("static_configs_%d", randName), pushData, globalStopChan)
		defer sg.stop()

		scrapeConfigPath := "test-scrape.yaml"
		var oldCfg, newCfg Config
		if err := oldCfg.parseData([]byte(oldCfgData), scrapeConfigPath); err != nil {
			t.Fatalf("cannot create old config: %s", err)
		}
		oldSws := oldCfg.getStaticScrapeWork()
		sg.update(oldSws)
		oldChangesCount := sg.changesCount.Get()

		if err := newCfg.parseData([]byte(newCfgData), scrapeConfigPath); err != nil {
			t.Fatalf("cannot create new config: %s", err)
		}
		doReload := (&newCfg).mustRestart(&oldCfg)
		if doReload != reloadExpected {
			t.Errorf("unexpected reload behaviour:\nexpected: %t\nactual: %t\n", reloadExpected, doReload)
		}
		newSws := newCfg.getStaticScrapeWork()
		sg.update(newSws)
		newChangesCount := sg.changesCount.Get()
		if (newChangesCount != oldChangesCount) != reloadExpected {
			t.Errorf("expected reload behaviour:\nexpected reload happen: %t\nactual reload happen: %t", reloadExpected, newChangesCount != oldChangesCount)
		}
	}
	f(`
scrape_configs:
- job_name: node-exporter
  static_configs:
    - targets:
        - localhost:8429`, `
scrape_configs:
- job_name: node-exporter
  static_configs:
    - targets:
        - localhost:8429`, false)
	f(`
scrape_configs:
- job_name: node-exporter
  static_configs:
    - targets:
        - localhost:8429`, `
scrape_configs:
- job_name: node-exporter
  static_configs:
    - targets:
        - localhost:8429
        - localhost:8428`, true)
	f(`
scrape_configs:
- job_name: node-exporter
  max_scrape_size: 1KiB
  static_configs:
    - targets:
        - localhost:8429`, `
scrape_configs:
- job_name: node-exporter
  max_scrape_size: 2KiB
  static_configs:
    - targets:
        - localhost:8429`, true)
}

func TestGetGlobalMetricMetadata_DeduplicationAndRecency(t *testing.T) {
	// Save and restore tsmGlobal.m
	tsmGlobal.mu.Lock()
	orig := make(map[*scrapeWork]*targetStatus)
	for k, v := range tsmGlobal.m {
		orig[k] = v
	}
	tsmGlobal.m = make(map[*scrapeWork]*targetStatus)
	tsmGlobal.mu.Unlock()
	defer func() {
		tsmGlobal.mu.Lock()
		tsmGlobal.m = orig
		tsmGlobal.mu.Unlock()
	}()

	now := time.Now().Unix()
	// Create two scrapeWorks with overlapping metric families
	sw1 := &scrapeWork{
		metadata: map[string]MetricMetadata{
			"foo": {MetricFamily: "foo", Type: "gauge", Help: "help1", Unit: "s", LastSeen: now - 10},
			"bar": {MetricFamily: "bar", Type: "counter", Help: "", Unit: "", LastSeen: now - 5},
		},
	}
	sw2 := &scrapeWork{
		metadata: map[string]MetricMetadata{
			"foo": {MetricFamily: "foo", Type: "gauge", Help: "help2", Unit: "s", LastSeen: now - 5},      // newer help
			"bar": {MetricFamily: "bar", Type: "counter", Help: "bar help", Unit: "", LastSeen: now - 20}, // older but with help
		},
	}
	tsmGlobal.mu.Lock()
	tsmGlobal.m[sw1] = &targetStatus{}
	tsmGlobal.m[sw2] = &targetStatus{}
	tsmGlobal.mu.Unlock()

	meta := GetGlobalMetricMetadata()
	if len(meta) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(meta))
	}
	if meta["foo"].Help != "help2" {
		t.Errorf("expected foo help2, got %q", meta["foo"].Help)
	}
	if meta["bar"].Help != "bar help" {
		t.Errorf("expected bar help 'bar help', got %q", meta["bar"].Help)
	}
}
