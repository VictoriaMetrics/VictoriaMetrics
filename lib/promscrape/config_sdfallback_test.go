package promscrape

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// sdResult is what a single service discovery config returns in one discovery round.
type sdResult struct {
	addrs []string
	err   error
}

func sdOK(addrs ...string) sdResult { return sdResult{addrs: addrs} }
func sdFail() sdResult              { return sdResult{err: errors.New("temporary discovery error")} }

// fakeSDConfig is a targetLabelsGetter returning a fixed sdResult.
type fakeSDConfig struct {
	r sdResult
}

func (f *fakeSDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
	if f.r.err != nil {
		return nil, f.r.err
	}
	labels := make([]*promutil.Labels, 0, len(f.r.addrs))
	for _, addr := range f.r.addrs {
		labels = append(labels, promutil.NewLabelsFromMap(map[string]string{"__address__": addr}))
	}
	return labels, nil
}

// TestGetScrapeWorkGeneric_PrevTargetsFallback covers what happens to a job's discovered
// targets across two consecutive discovery rounds when the job's service discovery
// configuration changes in between, typically on a config reload.
//
// getScrapeWorkGeneric runs once per discovery round for a single SD type (http_sd, consul_sd,
// ...). It is handed the targets that the previous round of the same SD type produced, and may
// fall back to them when discovery fails this round. The scenario under test is a job that is
// switched from one SD type to another, for example from http_sd_configs to file_sd_configs: in
// the round after the switch the job no longer has any http_sd_configs, and the http_sd targets
// from the previous round must disappear rather than be carried forward. The other cases pin
// the fallback's intended behaviour around that scenario: what survives a failed round, what a
// removed or renamed job inherits, and that SD types on the same job are independent.
//
// Every f() call is one such "previous round, then this round" pair and is independent of the
// others.
func TestGetScrapeWorkGeneric_PrevTargetsFallback(t *testing.T) {
	// f runs one round of getScrapeWorkGeneric for discoveryType ("http_sd_config" or
	// "consul_sd_config") against the config cfgData and compares the scrape targets it returns
	// with addrsExpected.
	//
	// prevJob and prevAddrs describe the previous round: the job prevJob had discovered the
	// targets prevAddrs. Pass an empty prevAddrs for "no previous round".
	//
	// results describes this round. Each SD config of the checked type in cfgData is answered
	// by the corresponding entry of results: the first config gets results[0], the second gets
	// results[1], and so on. A job in cfgData with no SD configs of the checked type asks for
	// nothing, so results is ignored for it.
	f := func(discoveryType, prevJob string, prevAddrs []string, cfgData string, results []sdResult, addrsExpected []string) {
		t.Helper()

		parse := func(data string) *Config {
			var cfg Config
			if err := cfg.parseData([]byte(data), "test.yml"); err != nil {
				t.Fatalf("cannot parse config: %s", err)
			}
			return &cfg
		}
		sdConfigsCount := func(sc *ScrapeConfig) int {
			switch discoveryType {
			case "http_sd_config":
				return len(sc.HTTPSDConfigs)
			case "consul_sd_config":
				return len(sc.ConsulSDConfigs)
			default:
				t.Fatalf("unsupported discoveryType %q", discoveryType)
				return 0
			}
		}
		visitWith := func(rs []sdResult) func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
			return func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
				n := sdConfigsCount(sc)
				if n > len(rs) {
					t.Fatalf("job %q has %d %s entries but only %d results were given", sc.JobName, n, discoveryType, len(rs))
				}
				for i := 0; i < n; i++ {
					visitor(&fakeSDConfig{r: rs[i]})
				}
			}
		}

		// Previous round: prevJob with a single SD config of the checked type discovering prevAddrs.
		var prev []*ScrapeWork
		if len(prevAddrs) > 0 {
			sdEntry := map[string]string{
				"http_sd_config":   "  http_sd_configs:\n  - url: http://sd.example.invalid/targets\n",
				"consul_sd_config": "  consul_sd_configs:\n  - server: consul.example.invalid:8500\n",
			}[discoveryType]
			cfgPrev := parse("scrape_configs:\n- job_name: " + prevJob + "\n" + sdEntry)
			prev = cfgPrev.getScrapeWorkGeneric(visitWith([]sdResult{sdOK(prevAddrs...)}), discoveryType, nil)
			if len(prev) != len(prevAddrs) {
				t.Fatalf("cannot build previous round; got %d scrape works; want %d", len(prev), len(prevAddrs))
			}
		}

		// This round.
		sws := parse(cfgData).getScrapeWorkGeneric(visitWith(results), discoveryType, prev)

		var got []string
		for _, sw := range sws {
			got = append(got, sw.ScrapeURL)
		}
		var want []string
		for _, addr := range addrsExpected {
			want = append(want, fmt.Sprintf("http://%s/metrics", addr))
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("unexpected scrape targets; got %v; want %v", got, want)
		}
	}

	cfgHTTP := `
scrape_configs:
- job_name: foo
  http_sd_configs:
  - url: http://sd.example.invalid/targets
`
	cfgTwoHTTP := `
scrape_configs:
- job_name: foo
  http_sd_configs:
  - url: http://a.example.invalid/targets
  - url: http://b.example.invalid/targets
`
	cfgFileOnly := `
scrape_configs:
- job_name: foo
  file_sd_configs:
  - files: [testdata/file_sd.json]
`
	cfgConsulOnly := `
scrape_configs:
- job_name: foo
  consul_sd_configs:
  - server: consul.example.invalid:8500
`
	cfgOtherJob := `
scrape_configs:
- job_name: bar
  static_configs:
  - targets: [host3:9100]
`
	cfgRenamed := `
scrape_configs:
- job_name: foo-renamed
  http_sd_configs:
  - url: http://sd.example.invalid/targets
`

	// Discovery succeeds: this round's targets replace the previous ones.
	f("http_sd_config", "foo", []string{"host1:9100", "host2:9100"}, cfgHTTP, []sdResult{sdOK("host3:9100")}, []string{"host3:9100"})

	// Discovery fails for every SD config of the job: the previous round's targets are preserved.
	// This is the purpose of the fallback and must keep working.
	f("http_sd_config", "foo", []string{"host1:9100", "host2:9100"}, cfgHTTP, []sdResult{sdFail()}, []string{"host1:9100", "host2:9100"})

	// The job still exists but its http_sd_configs were removed (switched to file_sd_configs).
	// This is not a discovery error: the job must yield no http_sd targets. Before the fix the
	// previous round's targets were preserved forever and kept being scraped until restart.
	f("http_sd_config", "foo", []string{"host1:9100", "host2:9100"}, cfgFileOnly, nil, nil)

	// The job was removed entirely: nothing is preserved.
	f("http_sd_config", "foo", []string{"host1:9100", "host2:9100"}, cfgOtherJob, nil, nil)

	// One of two SD configs fails: only the successful config's targets are used, nothing from
	// the previous round is merged in (behaviour introduced by the fix for #9949).
	f("http_sd_config", "foo", []string{"host1:9100"}, cfgTwoHTTP, []sdResult{sdFail(), sdOK("b1:9100")}, []string{"b1:9100"})

	// A successful discovery response with zero targets is a valid result, not an error.
	f("http_sd_config", "foo", []string{"host1:9100"}, cfgHTTP, []sdResult{sdOK()}, nil)

	// Every SD config fails on the very first round: there is nothing to preserve.
	f("http_sd_config", "foo", nil, cfgHTTP, []sdResult{sdFail()}, nil)

	// Each SD type keeps its own previous round. Removing http_sd_configs from a job that also
	// has consul_sd_configs drops only the http_sd targets; the consul_sd targets are unaffected.
	f("http_sd_config", "foo", []string{"http1:9100"}, cfgConsulOnly, nil, nil)
	f("consul_sd_config", "foo", []string{"consul1:9100"}, cfgConsulOnly, []sdResult{sdOK("consul1:9100")}, []string{"consul1:9100"})

	// The previous round is keyed by job name: a renamed job with failing discovery does not
	// inherit the old job's targets.
	f("http_sd_config", "foo", []string{"host1:9100"}, cfgRenamed, []sdResult{sdFail()}, nil)
}
