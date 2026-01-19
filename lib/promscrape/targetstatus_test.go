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

func TestRegisterDroppedTargets(t *testing.T) {
	type opts struct {
		toRegister       []*promutil.Labels
		wantTotalTargets int
	}
	f := func(opts opts) {
		t.Helper()
		dtm := &droppedTargets{
			m: make(map[uint64]droppedTarget),
		}

		for _, originalLabels := range opts.toRegister {
			dtm.Register(originalLabels, nil, targetDropReasonRelabeling, nil)
		}
		got := dtm.getTotalTargets()
		if got != opts.wantTotalTargets {
			t.Fatalf("unexpected total targets: (-%d;+%d)", opts.wantTotalTargets, got)
		}
	}

	// duplicate annotations
	f(opts{
		toRegister: []*promutil.Labels{
			promutil.MustNewLabelsFromString(`{up="1",__meta_kubernetes_endpoints_annotation_updated="123"}`),
			promutil.MustNewLabelsFromString(`{up="1",__meta_kubernetes_endpoints_annotation_updated="125"}`),
			promutil.MustNewLabelsFromString(`{up="1",__meta_docker_annotation_some="5"}`),
		},
		wantTotalTargets: 2,
	})
	// collision with missing annotation
	f(opts{
		toRegister: []*promutil.Labels{
			promutil.MustNewLabelsFromString(`{up="1",pod="vmagent-0"}`),
			promutil.MustNewLabelsFromString(`{up="1",pod="vmagent-0",__meta_kubernetes_endpoints_annotation_updated="125"}`),
			promutil.MustNewLabelsFromString(`{up="1",__meta_docker_annotation_some="5"}`),
		},
		wantTotalTargets: 2,
	})

}

func Test_targetStatus_GetSizeFromLastScrape_RoundUp(t *testing.T) {
	// the formula is: (N + 1023) / 1024, to avoid using `math` and doing type conversion.
	f := func(n int, expect string) {
		t.Helper()

		ts := &targetStatus{
			scrapeResponseSize: n,
		}
		result := ts.getSizeFromLastScrape()
		if expect != result {
			t.Fatalf("unexpected result; got %s; want %s", result, expect)
		}
	}

	f(0, "never scraped")
	f(1, "1B")
	f(1023, "1023B")
	f(1024, "1KiB")
	f(1025, "2KiB")
	f(1024*1024, "1024KiB")
	f(1024*1024+1, "1025KiB")
	f(1024*1024+1023, "1025KiB")

	f(1024*1024*1024, "1048576KiB")
}
