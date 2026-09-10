package promscrape

import (
	"errors"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// fakeSDConfig is a targetLabelsGetter that returns fixed labels or a fixed error.
type fakeSDConfig struct {
	labels []*promutil.Labels
	err    error
}

func (f *fakeSDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
	return f.labels, f.err
}

func mustParseConfig(t *testing.T, data string) *Config {
	t.Helper()
	var cfg Config
	if err := cfg.parseData([]byte(data), "test.yml"); err != nil {
		t.Fatalf("cannot parse config: %s", err)
	}
	return &cfg
}

// TestGetScrapeWorkGenericDropsTargetsWhenSDConfigsRemoved verifies that a job which no longer
// has any service-discovery config of a given type (e.g. http_sd_configs was removed from the job
// on config reload) does NOT keep the targets discovered by the removed config.
//
// The fallback in getScrapeWorkGeneric that preserves the previous round's targets exists for
// the case "every SD config of the job failed this round" (temporary discovery error). A job with
// zero SD configs of that type is not a discovery error: it must yield zero targets of that type,
// otherwise removing http_sd_configs from a job (or switching a job from http_sd to file_sd) keeps
// the old http_sd targets being scraped until the process is restarted.
func TestGetScrapeWorkGenericDropsTargetsWhenSDConfigsRemoved(t *testing.T) {
	labels := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{"__address__": "host1:9100"}),
		promutil.NewLabelsFromMap(map[string]string{"__address__": "host2:9100"}),
	}

	// Round 1: job "foo" has one http_sd config which discovers two targets.
	cfgWithSD := mustParseConfig(t, `
scrape_configs:
- job_name: foo
  http_sd_configs:
  - url: http://sd.example.invalid/targets
`)
	visitOne := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		visitor(&fakeSDConfig{labels: labels})
	}
	prev := cfgWithSD.getScrapeWorkGeneric(visitOne, "http_sd_config", nil)
	if len(prev) != 2 {
		t.Fatalf("round 1: expected 2 scrape works, got %d", len(prev))
	}

	// Round 2 (the bug): the same job still exists but its http_sd_configs were removed
	// (it now uses file_sd_configs). The real http_sd visitor finds nothing to visit.
	cfgWithoutSD := mustParseConfig(t, `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd.json]
`)
	sws := cfgWithoutSD.getHTTPDScrapeWork(prev)
	if len(sws) != 0 {
		t.Fatalf("job without http_sd_configs must yield 0 http_sd scrape works, got %d (previous round's targets were preserved): %v",
			len(sws), scrapeURLs(sws))
	}

	// Control 1: the job was removed entirely -> nothing is preserved (this already works).
	cfgNoJob := mustParseConfig(t, `
scrape_configs:
- job_name: bar
  static_configs:
  - targets: [host3:9100]
`)
	if sws := cfgNoJob.getHTTPDScrapeWork(prev); len(sws) != 0 {
		t.Fatalf("removed job must yield 0 scrape works, got %d", len(sws))
	}

	// Control 2: the job still has its http_sd config but discovery FAILS this round ->
	// the previous targets must be preserved (the intended purpose of the fallback).
	visitFailing := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		visitor(&fakeSDConfig{err: errors.New("temporary discovery error")})
	}
	if sws := cfgWithSD.getScrapeWorkGeneric(visitFailing, "http_sd_config", prev); len(sws) != 2 {
		t.Fatalf("failed discovery must preserve the previous 2 scrape works, got %d", len(sws))
	}
}

func scrapeURLs(sws []*ScrapeWork) []string {
	urls := make([]string, 0, len(sws))
	for _, sw := range sws {
		urls = append(urls, sw.ScrapeURL)
	}
	return urls
}
