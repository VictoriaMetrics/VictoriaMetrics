package promrelabel

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkApplyRelabelConfigs(b *testing.B) {
	b.Run("replace-label-copy", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "replace",
				SourceLabels: []string{"id"},
				TargetLabel:  "__name__",
				Regex:        defaultRegexForRelabelConfig,
				Replacement:  "$1",
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "foobar-random-string-here"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("replace-set-label", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:      "replace",
				TargetLabel: "__name__",
				Regex:       defaultRegexForRelabelConfig,
				Replacement: "foobar",
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "foobar" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "foobar"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("replace-add-label", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:      "replace",
				TargetLabel: "aaa",
				Regex:       defaultRegexForRelabelConfig,
				Replacement: "foobar",
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 2 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 2, labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "aaa" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "aaa"))
				}
				if labels[1].Value != "foobar" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar"))
				}
			}
		})
	})
	b.Run("replace-mismatch", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "replace",
				SourceLabels: []string{"non-existing-label"},
				TargetLabel:  "id",
				Regex:        regexp.MustCompile("(foobar)-.*"),
				Replacement:  "$1",
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("replace-match", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "replace",
				SourceLabels: []string{"id"},
				TargetLabel:  "id",
				Regex:        regexp.MustCompile("(foobar)-.*"),
				Replacement:  "$1",
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar"))
				}
			}
		})
	})
	b.Run("drop-mismatch", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "drop",
				SourceLabels: []string{"non-existing-label"},
				Regex:        regexp.MustCompile("(foobar)-.*"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("drop-match", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "drop",
				SourceLabels: []string{"id"},
				Regex:        regexp.MustCompile("(foobar)-.*"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("keep-mismatch", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "keep",
				SourceLabels: []string{"non-existing-label"},
				Regex:        regexp.MustCompile("(foobar)-.*"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("keep-match", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "keep",
				SourceLabels: []string{"id"},
				Regex:        regexp.MustCompile("(foobar)-.*"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("labeldrop-mismatch", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action: "labeldrop",
				Regex:  regexp.MustCompile("non-existing-label"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("labeldrop-match", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action: "labeldrop",
				Regex:  regexp.MustCompile("id"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 1 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 1, labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
			}
		})
	})
	b.Run("labelkeep-mismatch", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action: "labelkeep",
				Regex:  regexp.MustCompile("non-existing-label"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("labelkeep-match", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action: "labelkeep",
				Regex:  regexp.MustCompile("id"),
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != 1 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 1, labels))
				}
				if labels[0].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "id"))
				}
				if labels[0].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("hashmod", func(b *testing.B) {
		prcs := []ParsedRelabelConfig{
			{
				Action:       "hashmod",
				SourceLabels: []string{"id"},
				TargetLabel:  "id",
				Modulus:      23,
			},
		}
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = ApplyRelabelConfigs(labels, 0, prcs, true)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labels))
				}
				if labels[0].Name != "__name__" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "__name__"))
				}
				if labels[0].Value != "metric" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "metric"))
				}
				if labels[1].Name != "id" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "id"))
				}
				if labels[1].Value != "11" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "11"))
				}
			}
		})
	})
}
