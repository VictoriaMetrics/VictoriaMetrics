package streamaggr

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"gopkg.in/yaml.v2"
	"sigs.k8s.io/randfill"
)

func FuzzLoadFromData(f *testing.F) {
	f.Add([]byte("- interval: 1m\n  outputs: [count_samples]\n"))
	f.Add([]byte("- interval: 1m\n  outputs: [total]\n  match: '{job=\"test\"}'\n"))
	f.Add([]byte("- null\n"))
	f.Add([]byte("- interval: 1m\n  outputs: [count_samples]\n- null\n"))
	f.Add([]byte("- outputs: [unknown]\n"))
	f.Add([]byte("["))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		a, err := LoadFromData(data, func([]prompb.TimeSeries) {}, nil, "fuzz")
		if err == nil {
			a.MustStop()
		}
	})
}

func FuzzStructuredConfig(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("stream"))
	f.Add([]byte("%{HOME}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var cfg Config
		randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			Fill(&cfg)
		// Keep required fields valid so optional fields reach aggregation setup.
		cfg.Interval = "1m"
		cfg.Outputs = []string{"count_samples"}

		b, err := yaml.Marshal([]*Config{&cfg})
		if err != nil {
			return
		}
		a, err := LoadFromData(b, func([]prompb.TimeSeries) {}, nil, "fuzz")
		if err == nil {
			a.MustStop()
		}
	})
}
