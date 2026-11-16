package promrelabel

import (
	"fmt"
	"regexp"
	"testing"

	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/randfill"
)

func FuzzParseRelabelConfigsData(f *testing.F) {
	f.Add([]byte("- action: replace\n  source_labels: [instance]\n  target_label: job\n"))
	f.Add([]byte("- action: keep\n  source_labels: [job]\n  regex: [api, web]\n"))
	f.Add([]byte("- action: hashmod\n  source_labels: [instance]\n  target_label: shard\n  modulus: 4\n"))
	f.Add([]byte("- action: graphite\n  match: 'foo.*'\n  labels:\n    job: '$1'\n"))
	f.Add([]byte("- action: unknown\n"))
	f.Add([]byte("- regex: [\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		_, _ = ParseRelabelConfigsData(data)
	})
}

func FuzzRelabelConfig(f *testing.F) {
	const (
		nilIf = 1 << iota
		nilRegex
		nilSeparator
		nilReplacement
	)

	f.Add([]byte(""), 0)
	f.Add([]byte("instance"), 0)
	f.Add([]byte("%{HOME}"), 0)
	f.Add([]byte("instance"), nilIf)
	f.Add([]byte("instance"), nilRegex)
	f.Add([]byte("instance"), nilSeparator)
	f.Add([]byte("instance"), nilReplacement)
	f.Add([]byte("instance"), nilIf|nilRegex|nilSeparator|nilReplacement)

	f.Fuzz(func(t *testing.T, data []byte, n int) {
		if len(data) > 4096 {
			t.Skip()
		}

		var rc RelabelConfig
		randfill.NewFromGoFuzz(data).
			NilChance(0.2).
			MaxDepth(5).
			NumElements(0, 3).
			Funcs(func(ie *IfExpression, c randfill.Continue) {
				_ = ie.Parse(fmt.Sprintf("{fuzz_label=%q}", c.String(20)))
			}, func(regex *MultiLineRegex, c randfill.Continue) {
				regex.S = "(" + regexp.QuoteMeta("fuzz_"+c.String(20)) + ")"
			},
				// Generate valid RelabelConfig instead of purely random
				func(rc *RelabelConfig, c randfill.Continue) {
					rc.Action = "replace"
					rc.SourceLabels = []string{"fuzz_label"}
					rc.TargetLabel = "fuzz_label"
					// Each bit independently omits an optional field.
					if n&nilIf == 0 {
						rc.If = &IfExpression{}
						c.Fill(rc.If)
					}
					if n&nilRegex == 0 {
						rc.Regex = &MultiLineRegex{}
						c.Fill(rc.Regex)
					}
					if n&nilSeparator == 0 {
						rc.Separator = ptr.To(c.String(20))
					}
					if n&nilReplacement == 0 {
						rc.Replacement = ptr.To(c.String(20))
					}
				}).
			Fill(&rc)

		b, _ := yaml.Marshal([]RelabelConfig{rc})
		_, _ = ParseRelabelConfigsData(b)
	})
}
