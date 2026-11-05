package promscrape

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/randfill"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func FuzzUnmarshalConfig(f *testing.F) {
	f.Add([]byte("scrape_configs:\n- job_name: test\n  static_configs:\n  - targets: [localhost:8428]\n"))
	f.Add([]byte("global:\n  scrape_interval: 15s\nscrape_configs:\n- job_name: test\n  scheme: https\n"))
	f.Add([]byte("scrape_configs: ["))
	f.Add([]byte("unknown_field: true\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		for _, strict := range []bool{false, true} {
			var cfg Config
			_ = cfg.unmarshal(data, strict)
		}
	})
}

func FuzzParseConfig(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("scrape"))
	f.Add([]byte("%{HOME}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		// Random configs may contain discovery and auth file paths; only load their YAML.
		var sc ScrapeConfig
		randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			Funcs(func(d *promutil.Duration, c randfill.Continue) {
				d.D = time.Duration(1+c.Intn(60)) * time.Second
			}, func(labels *promutil.Labels, c randfill.Continue) {
				labels.InitFromMap(map[string]string{"fuzz_label": c.String(20)})
			}, func(u *proxy.URL, c randfill.Continue) {
				*u = *proxy.MustNewURL("http://localhost:8428/" + url.PathEscape(c.String(20)))
			}, func(ie *promrelabel.IfExpression, c randfill.Continue) {
				_ = ie.Parse(fmt.Sprintf("{fuzz_label=%q}", c.String(20)))
			}, func(regex *promrelabel.MultiLineRegex, c randfill.Continue) {
				regex.S = "(" + regexp.QuoteMeta("fuzz_"+c.String(20)) + ")"
			}).
			Fill(&sc)

		b, err := yaml.Marshal([]*ScrapeConfig{&sc})
		if err != nil {
			t.Skip()
		}
		var decoded []*ScrapeConfig
		if err := yaml.UnmarshalStrict(b, &decoded); err != nil {
			t.Skip()
		}
		dir := t.TempDir()
		const filename = "scrape.yml"
		const fileMode = 0o600
		if err := os.WriteFile(filepath.Join(dir, filename), b, fileMode); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadScrapeConfigFiles(dir, []string{filename}, true)
		if err != nil {
			t.Skip()
		}
		if len(loaded) != 1 || loaded[0] == nil {
			t.Fatal("expected one loaded scrape config")
		}

		// Attempt to build a scrape work config from the loaded scrape config. Must not panic, but may return an error.
		_, _ = getScrapeWorkConfig(loaded[0], dir, &GlobalConfig{})
	})
}
