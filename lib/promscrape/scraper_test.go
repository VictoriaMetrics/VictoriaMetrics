package promscrape

import (
	"fmt"
	"math/rand"
	"testing"

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
