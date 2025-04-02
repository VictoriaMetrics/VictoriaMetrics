package promscrape

import (
	"bytes"
	"encoding/json"
	"reflect"
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

	type activeTarget struct {
		DiscoveredLabels map[string]string `json:"discoveredLabels"`
		ScrapePool       string            `json:"scrapePool"`
	}
	f := func(scrapePoolFilter string, exp []activeTarget) {
		t.Helper()
		b := &bytes.Buffer{}
		tsm.WriteActiveTargetsJSON(b, scrapePoolFilter)

		var got []activeTarget
		if err := json.Unmarshal(b.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, exp) {
			t.Fatalf("unexpected response; \ngot\n %s; \nwant\n %s", got, exp)
		}
	}

	f("", []activeTarget{
		{ScrapePool: "foo", DiscoveredLabels: map[string]string{"__address__": "host1:80"}},
		{ScrapePool: "bar", DiscoveredLabels: map[string]string{"__address__": "host2:80"}},
	})
	f("foo", []activeTarget{
		{ScrapePool: "foo", DiscoveredLabels: map[string]string{"__address__": "host1:80"}},
	})
	f("bar", []activeTarget{
		{ScrapePool: "bar", DiscoveredLabels: map[string]string{"__address__": "host2:80"}},
	})
	f("unknown", []activeTarget{})
}
