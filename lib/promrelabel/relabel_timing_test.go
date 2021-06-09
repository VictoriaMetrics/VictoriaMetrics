package promrelabel

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkApplyRelabelConfigs(b *testing.B) {
	b.Run("replace-label-copy", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: replace
  source_labels: [id]
  target_label: __name__
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: replace
  target_label: __name__
  replacement: foobar
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: replace
  target_label: aaa
  replacement: foobar
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: replace
  source_labels: ["non-existing-label"]
  target_label: id
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("replace-match-regex", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: replace
  source_labels: [id]
  target_label: id
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: drop
  source_labels: ["non-existing-label"]
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: drop
  source_labels: [id]
  regex: yes
`)
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "yes",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("drop-match-regexp", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: drop
  source_labels: [id]
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("keep-mismatch", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: keep
  source_labels: ["non-existing-label"]
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("keep-match", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: keep
  source_labels: [id]
  regex: yes
`)
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: "metric",
			},
			{
				Name:  "id",
				Value: "yes",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0, true)
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
				if labels[1].Value != "yes" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "yes"))
				}
			}
		})
	})
	b.Run("keep-match-regexp", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: keep
  source_labels: [id]
  regex: "(foobar)-.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: labeldrop
  regex: "non-existing-label"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: labeldrop
  regex: id
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("labeldrop-match-prefix", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labeldrop
  regex: "id.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("labeldrop-match-regexp", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labeldrop
  regex: ".*id.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
		pcs := mustParseRelabelConfigs(`
- action: labelkeep
  regex: "non-existing-label"
`)
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
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 0 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 0, labels))
				}
			}
		})
	})
	b.Run("labelkeep-match", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelkeep
  regex: id
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("labelkeep-match-prefix", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelkeep
  regex: "id.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("labelkeep-match-regexp", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelkeep
  regex: ".*id.*"
`)
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
				labels = pcs.Apply(labels, 0, true)
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
	b.Run("labelmap-mismatch", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelmap
  regex: "a(.*)"
`)
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "foo",
				Value: "bar",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 1 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 3, labels))
				}
				if labels[0].Name != "foo" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "foo"))
				}
				if labels[0].Value != "bar" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "bar"))
				}
			}
		})
	})
	b.Run("labelmap-match-remove-prefix", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelmap
  regex: "a(.*)"
`)
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "aabc",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 2 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 3, labels))
				}
				if labels[0].Name != "aabc" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "aabc"))
				}
				if labels[0].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "foobar-random-string-here"))
				}
				if labels[1].Name != "abc" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "abc"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("labelmap-match-regexp", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: labelmap
  regex: "(.*)bc"
`)
		labelsOrig := []prompbmarshal.Label{
			{
				Name:  "aabc",
				Value: "foobar-random-string-here",
			},
		}
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0, true)
				if len(labels) != 2 {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), 3, labels))
				}
				if labels[0].Name != "aa" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[0].Name, "aa"))
				}
				if labels[0].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[0].Value, "foobar-random-string-here"))
				}
				if labels[1].Name != "aabc" {
					panic(fmt.Errorf("unexpected label name; got %q; want %q", labels[1].Name, "aabc"))
				}
				if labels[1].Value != "foobar-random-string-here" {
					panic(fmt.Errorf("unexpected label value; got %q; want %q", labels[1].Value, "foobar-random-string-here"))
				}
			}
		})
	})
	b.Run("hashmod", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: hashmod
  source_labels: [id]
  target_label: id
  modulus: 23
`)
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
				labels = pcs.Apply(labels, 0, true)
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

func mustParseRelabelConfigs(config string) *ParsedConfigs {
	pcs, err := ParseRelabelConfigsData([]byte(config), false)
	if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}
	return pcs
}
