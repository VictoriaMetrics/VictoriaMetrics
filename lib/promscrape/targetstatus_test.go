package promscrape

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestWriteActiveTargetsJSON(t *testing.T) {
	tsm := newTargetStatusMap()
	tsm.Register(&scrapeWork{
		Config: &ScrapeWork{
			jobNameOriginal: "foo",
			OriginalLabels: promutil.NewLabelsFromMap(map[string]string{
				"__address__": "host1:80",
			}),
		},
	})
	tsm.Register(&scrapeWork{
		Config: &ScrapeWork{
			jobNameOriginal: "bar",
			OriginalLabels: promutil.NewLabelsFromMap(map[string]string{
				"__address__": "host2:80",
			}),
		},
	})

	f := func(scrapePoolFilter, exp string) {
		t.Helper()
		b := &bytes.Buffer{}
		tsm.WriteActiveTargetsJSON(b, scrapePoolFilter)
		got := b.String()
		if got != exp {
			t.Fatalf("unexpected response; \ngot\n %s; \nwant\n %s", got, exp)
		}
	}

	f("", `[{"discoveredLabels":{"__address__":"host1:80"},"labels":{},"scrapePool":"foo","scrapeUrl":"","lastError":"","lastScrape":"1970-01-01T01:00:00+01:00","lastScrapeDuration":0,"lastSamplesScraped":0,"health":"down"},{"discoveredLabels":{"__address__":"host2:80"},"labels":{},"scrapePool":"bar","scrapeUrl":"","lastError":"","lastScrape":"1970-01-01T01:00:00+01:00","lastScrapeDuration":0,"lastSamplesScraped":0,"health":"down"}]`)
	f("foo", `[{"discoveredLabels":{"__address__":"host1:80"},"labels":{},"scrapePool":"foo","scrapeUrl":"","lastError":"","lastScrape":"1970-01-01T01:00:00+01:00","lastScrapeDuration":0,"lastSamplesScraped":0,"health":"down"}]`)
	f("bar", `[{"discoveredLabels":{"__address__":"host2:80"},"labels":{},"scrapePool":"bar","scrapeUrl":"","lastError":"","lastScrape":"1970-01-01T01:00:00+01:00","lastScrapeDuration":0,"lastSamplesScraped":0,"health":"down"}]`)
	f("unknown", `[]`)
}
