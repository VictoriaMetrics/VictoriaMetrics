package promrelabel

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkSanitizeMetricName(b *testing.B) {
	for _, name := range []string{"", "foo", "foo-bar-baz", "http_requests_total"} {
		b.Run(name, func(b *testing.B) {
			benchmarkSanitizeMetricName(b, name)
		})
	}
}

func benchmarkSanitizeMetricName(b *testing.B, name string) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sanitizedName := SanitizeMetricName(name)
			GlobalSink += len(sanitizedName)
		}
	})
}

func BenchmarkSanitizeLabelName(b *testing.B) {
	for _, name := range []string{"", "foo", "foo-bar-baz", "http_requests_total"} {
		b.Run(name, func(b *testing.B) {
			benchmarkSanitizeLabelName(b, name)
		})
	}
}

func benchmarkSanitizeLabelName(b *testing.B, name string) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sanitizedName := SanitizeLabelName(name)
			GlobalSink += len(sanitizedName)
		}
	})
}

var GlobalSink int

func BenchmarkMatchRegexPrefixDotPlusMatchOptimized(b *testing.B) {
	const pattern = "^foo.+$"
	const s = "foobar"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotPlusMatchUnoptimized(b *testing.B) {
	const pattern = "^foo.+$"
	const s = "foobar"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !re.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotPlusMismatchOptimized(b *testing.B) {
	const pattern = "^foo.+$"
	const s = "xfoobar"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotPlusMismatchUnoptimized(b *testing.B) {
	const pattern = "^foo.+$"
	const s = "xfoobar"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if re.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotStarMatchOptimized(b *testing.B) {
	const pattern = "^foo.*$"
	const s = "foobar"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotStarMatchUnoptimized(b *testing.B) {
	const pattern = "^foo.*$"
	const s = "foobar"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !re.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotStarMismatchOptimized(b *testing.B) {
	const pattern = "^foo.*$"
	const s = "xfoobar"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexPrefixDotStarMismatchUnoptimized(b *testing.B) {
	const pattern = "^foo.*$"
	const s = "xfoobar"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if re.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexSingleValueMatchOptimized(b *testing.B) {
	const pattern = "^foo$"
	const s = "foo"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexSingleValueMatchUnoptimized(b *testing.B) {
	const pattern = "^foo$"
	const s = "foo"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !re.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexSingleValueMismatchOptimized(b *testing.B) {
	const pattern = "^foo$"
	const s = "bar"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexSingleValueMismatchUnoptimized(b *testing.B) {
	const pattern = "^foo$"
	const s = "bar"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if re.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexOrValuesMatchOptimized(b *testing.B) {
	const pattern = "^(foo|bar|baz|abc)$"
	const s = "foo"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexOrValuesMatchUnoptimized(b *testing.B) {
	const pattern = "^(foo|bar|baz|abc)$"
	const s = "foo"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !re.MatchString(s) {
				panic(fmt.Errorf("unexpected string mismatch for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexOrValuesMismatchOptimized(b *testing.B) {
	const pattern = "^(foo|bar|baz|abc)"
	const s = "qwert"
	prc := newTestRegexRelabelConfig(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if prc.regex.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkMatchRegexOrValuesMismatchUnoptimized(b *testing.B) {
	const pattern = "^(foo|bar|baz|abc)$"
	const s = "qwert"
	re := regexp.MustCompile(pattern)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if re.MatchString(s) {
				panic(fmt.Errorf("unexpected string match for pattern=%q, s=%q", pattern, s))
			}
		}
	})
}

func BenchmarkApplyRelabelConfigs(b *testing.B) {
	b.Run("kubernetes", func(b *testing.B) {
		// See https://github.com/Arnoways/prometheus/blob/d521933053bdf68d252e365da9376706d04addcc/model/relabel/relabel_test.go#L505
		pcs := mustParseRelabelConfigs(`
- source_labels:
  - __meta_kubernetes_pod_container_port_name
  regex: .*-metrics
  action: keep
- source_labels:
  - __meta_kubernetes_pod_label_name
  action: drop
  regex: ""
- source_labels:
  - __meta_kubernetes_pod_phase
  regex: Succeeded|Failed
  action: drop
- source_labels:
  - __meta_kubernetes_pod_annotation_prometheus_io_scrape
  regex: "false"
  action: drop
- source_labels:
  - __meta_kubernetes_pod_annotation_prometheus_io_scheme
  target_label: __scheme__
  regex: (https?)
  replacement: $1
  action: replace
- source_labels:
  - __meta_kubernetes_pod_annotation_prometheus_io_path
  target_label: __metrics_path__
  regex: (.+)
  replacement: $1
  action: replace
- source_labels:
  - __address__
  - __meta_kubernetes_pod_annotation_prometheus_io_port
  target_label: __address__
  regex: (.+?)(\:\d+)?;(\d+)
  replacement: $1:$3
  action: replace
- regex: __meta_kubernetes_pod_annotation_prometheus_io_param_(.+)
  replacement: __param_$1
  action: labelmap
- regex: __meta_kubernetes_pod_label_prometheus_io_label_(.+)
  action: labelmap
- regex: __meta_kubernetes_pod_annotation_prometheus_io_label_(.+)
  action: labelmap
- source_labels:
  - __meta_kubernetes_namespace
  - __meta_kubernetes_pod_label_name
  separator: /
  target_label: job
  replacement: $1
  action: replace
- source_labels:
  - __meta_kubernetes_namespace
  target_label: namespace
  action: replace
- source_labels:
  - __meta_kubernetes_pod_name
  target_label: pod
  action: replace
- source_labels:
  - __meta_kubernetes_pod_container_name
  target_label: container
  action: replace
- source_labels:
  - __meta_kubernetes_pod_name
  - __meta_kubernetes_pod_container_name
  - __meta_kubernetes_pod_container_port_name
  separator: ':'
  target_label: instance
  action: replace
- target_label: cluster
  replacement: dev-us-central-0
- source_labels:
  - __meta_kubernetes_namespace
  regex: hosted-grafana
  action: drop
- source_labels:
  - __address__
  target_label: __tmp_hash
  modulus: 3
  action: hashmod
- source_labels:
  - __tmp_hash
  regex: ^0$
  action: keep
- regex: __tmp_hash
  action: labeldrop
`)
		labelsOrig := labelsFromStrings(
			"__address__", "10.132.183.40:80",
			"__meta_kubernetes_namespace", "loki-boltdb-shipper",
			"__meta_kubernetes_pod_annotation_promtail_loki_boltdb_shipper_hash", "50523b9759094a144adcec2eae0aa4ad",
			"__meta_kubernetes_pod_annotationpresent_promtail_loki_boltdb_shipper_hash", "true",
			"__meta_kubernetes_pod_container_init", "false",
			"__meta_kubernetes_pod_container_name", "promtail",
			"__meta_kubernetes_pod_container_port_name", "http-metrics",
			"__meta_kubernetes_pod_container_port_number", "80",
			"__meta_kubernetes_pod_container_port_protocol", "TCP",
			"__meta_kubernetes_pod_controller_kind", "DaemonSet",
			"__meta_kubernetes_pod_controller_name", "promtail-loki-boltdb-shipper",
			"__meta_kubernetes_pod_host_ip", "10.128.0.178",
			"__meta_kubernetes_pod_ip", "10.132.183.40",
			"__meta_kubernetes_pod_label_controller_revision_hash", "555b77cd7d",
			"__meta_kubernetes_pod_label_name", "promtail-loki-boltdb-shipper",
			"__meta_kubernetes_pod_label_pod_template_generation", "45",
			"__meta_kubernetes_pod_labelpresent_controller_revision_hash", "true",
			"__meta_kubernetes_pod_labelpresent_name", "true",
			"__meta_kubernetes_pod_labelpresent_pod_template_generation", "true",
			"__meta_kubernetes_pod_name", "promtail-loki-boltdb-shipper-jgtr7",
			"__meta_kubernetes_pod_node_name", "gke-dev-us-central-0-main-n2s8-2-14d53341-9hkr",
			"__meta_kubernetes_pod_phase", "Running",
			"__meta_kubernetes_pod_ready", "true",
			"__meta_kubernetes_pod_uid", "4c586419-7f6c-448d-aeec-ca4fa5b05e60",
			"__metrics_path__", "/metrics",
			"__scheme__", "http",
			"__scrape_interval__", "15s",
			"__scrape_timeout__", "10s",
			"job", "kubernetes-pods",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				if len(labels) != 0 {
					panic(fmt.Errorf("BUG: expecting empty labels"))
				}
			}
		})
	})
	b.Run("replace-label-copy", func(b *testing.B) {
		pcs := mustParseRelabelConfigs(`
- action: replace
  source_labels: [id]
  target_label: __name__
`)
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
				if len(labels) != len(labelsOrig) {
					panic(fmt.Errorf("unexpected number of labels; got %d; want %d; labels:\n%#v", len(labels), len(labelsOrig), labelsOrig))
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
  regex: "yes"
`)
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "yes",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
  regex: "yes"
`)
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "yes",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"foo", "bar",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"aabc", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"aabc", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
		labelsOrig := labelsFromStrings(
			"__name__", "metric",
			"id", "foobar-random-string-here",
		)
		b.ReportAllocs()
		b.SetBytes(1)
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompbmarshal.Label
			for pb.Next() {
				labels = append(labels[:0], labelsOrig...)
				labels = pcs.Apply(labels, 0)
				labels = FinalizeLabels(labels[:0], labels)
				SortLabels(labels)
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
	pcs, err := ParseRelabelConfigsData([]byte(config))
	if err != nil {
		panic(fmt.Errorf("unexpected error: %w", err))
	}
	return pcs
}

func labelsFromStrings(ss ...string) []prompbmarshal.Label {
	labelsLen := len(ss) / 2
	labels := make([]prompbmarshal.Label, 0, labelsLen)
	for i := 0; i < len(ss); i += 2 {
		labels = append(labels, prompbmarshal.Label{
			Name:  ss[i],
			Value: ss[i+1],
		})
	}
	return labels
}
