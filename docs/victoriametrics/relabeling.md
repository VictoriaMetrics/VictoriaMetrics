---
weight: 37
title: Relabeling cookbook
menu:
  docs:
    parent: 'victoriametrics'
    weight: 37
aliases:
- /relabeling.html
---

The relabeling cookbook provides practical examples and patterns for transforming your metrics data as it flows through VictoriaMetrics, helping you control what gets collected and how it's labeled.

VictoriaMetrics and vmagent support Prometheus-style relabeling with [extra features](https://docs.victoriametrics.com/vmagent/#relabeling-enhancements) to enhance the functionality.

**Target-level relabeling** is applied during [service discovery](https://docs.victoriametrics.com/sd_configs/#prometheus-service-discovery) and affects the targets (which will be scraped), their labels and all the metrics scraped from them:

- [Drop targets](#how-to-drop-discovered-targets): Filter out unwanted targets from being scraped based on their labels
- [Configure scrape URLs](#how-to-modify-scrape-urls-in-targets): Change which URL is used to fetch metrics from each target
- [Add or update static labels](#how-to-add-labels-to-scrape-targets): Attach or update constant label values to all metrics from specific targets
- [Copy labels](#how-to-copy-labels-in-scrape-targets): Duplicate label values from one label to another
- [Modify instance/job labels](#how-to-modify-instance-and-job): Change the default `instance` and `job` labels for discovered targets
- [Extract label parts](#how-to-extract-label-parts): Parse and extract portions of label values into new labels
- [Remove prefixes](#how-to-remove-prefixes-from-target-label-names): Clean up label names by removing common prefixes
- [Remove labels](#how-to-remove-labels-from-targets): Delete specific labels from discovered targets

Note: All the target-level labels which are not prefixed with `__` are automatically added to all the metrics scraped from targets.

**Metric-level relabeling** is applied after metrics are scraped and affects the individual metrics:

- [Drop metrics](#how-to-drop-metrics-during-scrape): Filter out specific metrics to reduce cardinality and storage requirements
- [Rename metrics](#how-to-rename-scraped-metrics): Change metric names to follow naming conventions or standards
- [Add metric labels](#how-to-add-labels-to-scraped-metrics): Attach additional labels to scraped metrics for better querying
- [Change label values](#how-to-change-label-values-in-scraped-metrics): Modify existing label values to normalize or transform them
- [Remove metric labels](#how-to-remove-labels-from-scraped-metrics): Delete specific labels from scraped metrics
- [Remove labels with conditions](#how-to-remove-labels-from-metrics-subset): Delete labels only from metrics matching specific criteria

See also [relabeling docs at vmagent](https://docs.victoriametrics.com/vmagent/#relabeling).

## How to remove labels from metrics subset

You can remove certain labels from some metrics without affecting other labels by using the `if` parameter with `labeldrop` action. The `if` parameter is a [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) - it looks at the metric name and labels of each scraped time series.

To demonstrate this:

- This config below removes the `cpu` and `mode` labels, but only from the `node_cpu_seconds_total` metric where `mode="idle"`:
  ```yaml
  metric_relabel_configs:
    - action: labeldrop
      if: 'node_cpu_seconds_total{mode="idle"}'
      regex: "cpu|mode"
  ```

## How to rename scraped metrics

The metric name is actually the value of a special label called `__name__` (see [Key Concepts](https://docs.victoriametrics.com/keyconcepts/#labels)). So renaming a metric is performed in the same way as changing a label value. Let's take some examples:

- Rename `foo` metric to `bar` across all the scraped metrics:
  ```yaml
  metric_relabel_configs:
  - if: 'foo'
    replacement: bar
    target_label: __name__
  ```

- Rename all metrics starting with `foo_` to start with `bar_` instead (e.g. `foo_count` → `bar_count`):
  ```yaml
  metric_relabel_configs:
  - source_labels: [__name__]
    regex: 'foo_(.*)'
    replacement: bar_$1
    target_label: __name__
  ```

- Replace all dashes (`-`) in metric names with underscores (`_`) (e.g. `foo-bar-baz` → `foo_bar_baz`):
  ```yaml
  metric_relabel_configs:
  - source_labels: [__name__]
    action: replace_all
    regex: '-'
    replacement: '_'
    target_label: __name__
  ```

## How to add labels to scraped metrics

You can add custom labels to scraped metrics using `target_label` to set the label name and the `replacement` field to set the label value. For example:

- Add a `foo="bar"` label to all scraped metrics:
  ```yaml
  metric_relabel_configs:
  - target_label: foo
    replacement: bar
  ```

- Add a `foo="bar"` label only for metrics matching `{job=~"my-app-.*",env!="dev"}` series selector:
  ```yaml
  metric_relabel_configs:
  - if: '{job=~"my-app-.*",env!="dev"}'
    target_label: foo
    replacement: bar
  ```

## How to change label values in scraped metrics

To change the label values of scraped metrics, we use the following fields:
- `target_label`: the label we want to modify (if it exists) or create,
- `source_labels`: the label(s) whose values are used to compute the new value for `target_label`,
- `replacement`: the value that will be computed and assigned to the `target_label`.

Below are a few illustrations:

- Add `foo_` prefix to all values of the `job` label across all scraped metrics:
  ```yaml
  metric_relabel_configs:
  - source_labels: [job]
    target_label: job
    replacement: foo_$1
  ```

- Add `foo_` prefix to `job` label values only for metrics matching `{job=~"my-app-.*",env!="dev"}`:
  ```yaml
  metric_relabel_configs:
  - if: '{job=~"my-app-.*",env!="dev"}'
    source_labels: [job]
    target_label: job
    replacement: foo_$1
  ```

## How to remove labels from scraped metrics

Removing labels from scraped metrics is a good idea to avoid [high cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality) and [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) issues.

This can be done with either of the following actions:
- `action: labeldrop`: drops labels with names matching the given `regex` option
- `action: labelkeep`: drops labels with names not matching the given `regex` option

Let's see this in action:

- Remove labels with names starting with `foo_` prefix from all scraped metrics:
  ```yaml
  metric_relabel_configs:
  - action: labeldrop
    regex: "foo_.*"
  ```

The `regex` option must match the whole label name from start to end, not just a part of it.

Note that:

- Labels that start with `__` are removed automatically after relabeling, so you don't need to drop them with relabeling rules.

## How to drop metrics during scrape

All examples above work at the label level: adding, dropping, or changing label values of scraped metrics. You can also drop entire metrics. This is especially beneficial for metrics that result in [high cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality) or [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate).

Instead of `labeldrop` or `labelkeep` actions, we use `drop` or `keep` actions in the `metric_relabel_configs` section:

- `action: drop`: drops all metrics that match the `if` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering)
- `action: keep`: drops all metrics that don't match the `if` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering)

For example, the following config drops all metrics with names starting with `foo_`:

```yaml
metric_relabel_configs:
- if: '{__name__=~"foo_.*"}'
  action: drop
```

Note that the relabeling config is specified under the `metric_relabel_configs` section instead of `relabel_configs` section. They serve different purposes:

- The `scrape_configs[].relabel_configs` apply before scraping, modifying or filtering targets. Any changes here affect all metrics from that target.
- The `scrape_configs[].metric_relabel_configs` apply after scraping, modifying or filtering individual metrics.

## How to remove labels from targets

To remove some labels from targets discovered by the scrape job, use either:
- `action: labeldrop`: drops labels with names matching the given `regex` option
- `action: labelkeep`: drops labels with names not matching the given `regex` option

For example:

- The job below discovers pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs), extracts pod-level labels, prefixes them with `foo_`, adds them to all metrics and finally drops all labels with the `foo_bar_` prefix:
  ```yaml
  scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - action: labelmap
      regex: "__meta_kubernetes_pod_label_(.+)"
      replacement: "foo_$1"
    - action: labeldrop
      regex: "foo_bar_.*"
  ```

Note that:

- Do not remove `instance` and `job` labels, since this may result in duplicate scrape targets with identical sets of labels.
- The `regex` option must match the whole label name from start to end, not just a part of it.
- Labels that start with `__` are removed automatically after relabeling, so you don't need to drop them with relabeling rules.

## How to remove labels from a subset of targets

To remove some target-labels from a subset of discovered targets, use the `if` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering) with `action: labeldrop` or `action: labelkeep` relabeling rule.

As an illustration:

- The job below discovers pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs), extracts pod-level labels, adds them to all metrics with prefix `foo_` and finally drops all labels whose names start with `foo_bar_` for **only targets matching `{__address__=~"pod123.+"}` selector**:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - action: labelmap
    regex: "__meta_kubernetes_pod_label_(.+)"
    replacement: "foo_$1"
  - action: labeldrop
    if: '{__address__=~"pod123.+"}'
    regex: "foo_bar_.*"
```

## How to remove prefixes from target label names

You can modify target-labels including removing prefixes with the `action: labelmap` option.

For example, [Kubernetes service discovery](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) automatically adds special `__meta_kubernetes_pod_label_<labelname>` labels for each pod-level label. 

All labels with the prefix `__` will be dropped automatically. To extract and keep only the `<labelname>` part of this special label, you can use `action: labelmap` combined with `regex` and `replacement` options:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - action: labelmap
    regex: "__meta_kubernetes_pod_label_(.+)"
    replacement: "$1"
```

The regex contains a capture group `(.+)`. This capture group can be referenced inside the `replacement` option with the `$N` syntax, such as `$1` for the first capture group.

This config will create a new label with the name extracted from the regex capture group `(.+)` for all metrics scraped from the discovered pods.

Note that:

- The `regex` option must match the whole label name from start to end, not just a part of it.

## How to extract label parts

Relabeling allows extracting parts from label values and storing them into arbitrary labels. This is performed with:

- `source_labels`: the label(s) whose values are used to compute the new value for `target_label`,
- `target_label`: the label we want to modify or create,
- `replacement`: the value that will be computed and assigned to the `target_label`,
- `regex`: the regular expression to be applied to the value of `source_labels`.

Let's take this case:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_container_name]
    regex: "[^/]+/(.+)"
    replacement: "abc_$1"
    target_label: xyz
```

The job above discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs), and performs these actions:

1. Extracts the value of `__meta_kubernetes_pod_container_name` label (e.g. `foo/bar`), 
2. Matches it against the regex `[^/]+/(.+)`, 
3. Computes the new value as `abc_$1` with `$1` capture from regex `(.+)`, 
4. Stores the result in the `xyz` label.

Note that:

- The `regex` option must match the whole label value from start to end, not just a part of it.
- If `source_labels` contains multiple labels, their values are joined with a `;` separator (customized by the `separator` option) before being matched against the `regex`.

## How to modify instance and job

`instance` and `job` labels are automatically added by single-node VictoriaMetrics and [vmagent](https://docs.victoriametrics.com/vmagent/) for each discovered target.

- The `job` label is set to the `job_name` value specified in the corresponding `scrape_config`.
- The `instance` label is set to the `host:port` part of the `__address__` label value after target-level relabeling. The `__address__` label value depends on the type of [service discovery](https://docs.victoriametrics.com/sd_configs/#supported-service-discovery-configs) and [can be overridden](https://docs.victoriametrics.com/sd_configs/#scrape_configs) during relabeling.

Modifying `instance` and `job` labels works like other target-labels by using `target_label` and `replacement` options:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - target_label: job
    replacement: foo
```

## How to modify scrape URLs in targets

URLs for scrape targets are composed of the following parts:

- Scheme (e.g. `http`, `https`) is available during target relabeling in a special label - `__scheme__`. By default, it's set to `http` but can be overridden either by specifying the `scheme` option at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs) level or by updating the `__scheme__` label during relabeling.
- Host and port (e.g. `host12:3456`) is available during target relabeling in a special label - `__address__`. Its value depends on the [service discovery type](https://docs.victoriametrics.com/sd_configs/#supported-service-discovery-configs). Sometimes this value needs to be modified. In this case, just update the `__address__` label during relabeling to the needed value. 
  - The port part is optional. If it is missing, it's automatically set depending on the scheme (`80` for `http` or `443` for `https`). The `host:port` part from the final `__address__` label is automatically set to the `instance` label. The `__address__` label can contain the full scrape URL (e.g. `http://host:port/metrics/path?query_args`). In this case the `__scheme__` and `__metrics_path__` labels are ignored.
- URL path (e.g. `/metrics`) is available during target relabeling in a special label - `__metrics_path__`. By default, it's set to `/metrics` and can be overridden either by specifying the `metrics_path` option at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs) level or by updating the `__metrics_path__` label during relabeling.
- Query args (e.g. `?foo=bar&baz=xyz`) are available during target relabeling in special labels with the `__param_` prefix. 
  - Take `?foo=bar&baz=xyz` for example. There will be two special labels: `__param_foo="bar"` and `__param_baz="xyz"`. The query args can be specified either via the `params` section at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs) or by updating/setting the corresponding `__param_*` labels during relabeling.

The resulting scrape URL looks like the following:

```go
<__scheme__> + "://" + <__address__> + <__metrics_path__> + <"?" + query_args_from_param_labels>
```

Given the scrape URL construction rules above, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) and constructs a per-target scrape URL as `https://<pod_name>/foo/bar?baz=<container_name>`:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  metrics_path: /foo/bar
  relabel_configs:
  - target_label: __scheme__
    replacement: https
  - source_labels: [__meta_kubernetes_pod_name]
    target_label: __address__
  - source_labels: [__meta_kubernetes_pod_container_name]
    target_label: __param_baz
```

## How to copy labels in scrape targets

Labels can be copied using the following options:

- `source_labels`: specifies which labels to copy from
- `target_label`: specifies the destination label to receive the value

The following config copies the `__meta_kubernetes_pod_name` label to the `pod` label for all discovered pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_name]
    target_label: pod
```

If `source_labels` contains multiple labels, their values are joined with a `;` delimiter by default. Use the `separator` option to change this delimiter.

For example, this config combines pod name and container port into the `host_port` label for all discovered pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_name, __meta_kubernetes_pod_container_port_number]
    separator: ":"
    target_label: host_port
```

## How to add labels to scrape targets

To add or update labels on scrape targets during discovery, use these options:

- `target_label`: specifies the label name to add or update
- `replacement`: specifies the value to assign to this label

For example, this config adds a `foo="bar"` label to all discovered pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - target_label: "foo"
    replacement: "bar"
```

If there is a conflict between target labels and metrics exported by the target (scrape-time labels), the `exported_` prefix is added to scrape-time labels.

To keep the scrape-time labels unchanged and let them override target labels, specify `honor_labels: true` in the scrape config. This gives priority to the labels from the scraped metrics.

For example, this config adds a `foo="bar"` label to all discovered pods, but if any pod already exports a `foo` label, that value will override the target label:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  honor_labels: true
  relabel_configs:
  - target_label: "foo"
    replacement: "bar"
```

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).

## How to drop discovered targets

To drop a particular discovered target, use the following options:

- `action: drop`: drops scrape targets with labels matching the `if` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering)
- `action: keep`: keeps scrape targets with labels matching the `if` series selector, while dropping all other targets

Here are examples of these options:

- This config discovers pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) and drops all pods with names starting with the `foo` prefix:
  ```yaml
  scrape_configs:
  - job_name: not_foo_pods
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - if: '{__meta_kubernetes_pod_name=~"foo.*"}'
      action: drop
  ```

- This config keeps only pods with names starting with the `foo` prefix:
  ```yaml
  scrape_configs:
  - job_name: foo_pods
    kubernetes_sd_configs:
    - role: pod
    relabel_configs:
    - if: '{__meta_kubernetes_pod_name=~"foo.*"}'
      action: keep
  ```

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).

## Useful tips for target relabeling

- Target relabeling can be debugged by clicking the `debug` link for a target on the `http://vmagent:8429/target` or `http://vmagent:8429/service-discovery` pages. See [Relabel Debug - vmagent](https://docs.victoriametrics.com/vmagent/#relabel-debug).
- Special labels with the `__` prefix are automatically added when discovering targets and removed after relabeling:
  - Meta-labels starting with the `__meta_` prefix. The specific sets of labels for each supported service discovery option are listed in [Prometheus Service Discovery](https://docs.victoriametrics.com/sd_configs/#prometheus-service-discovery).
  - Additional labels with the `__` prefix other than `__meta_` labels, such as [`__scheme__` or `__address__`](#how-to-modify-scrape-urls-in-targets).
  It is common practice to store temporary labels with names starting with `__` during target relabeling.
- All target-level labels are automatically added to all metrics scraped from targets.
- The list of discovered scrape targets with all discovered meta-labels is available on the `http://vmagent:8429/service-discovery` page for `vmagent` and on the `http://victoriametrics:8428/service-discovery` page for single-node VictoriaMetrics.
- The list of active targets with the final set of target-labels after relabeling is available on the `http://vmagent:8429/targets` page for `vmagent` and on the `http://victoriametrics:8428/targets` page for single-node VictoriaMetrics.

## Useful tips for metric relabeling

- Metric relabeling can be debugged on the `http://vmagent:8429/metric-relabel-debug` page. See [these docs](https://docs.victoriametrics.com/vmagent/#relabel-debug).
- All labels that start with the `__` prefix are automatically removed from metrics after relabeling. It is common practice to store temporary labels with names starting with `__` during metrics relabeling.
- All target-level labels are automatically added to all metrics scraped from targets, making them available during metrics relabeling.
- If too many labels are removed, different metrics might look the same — this can lead to duplicate time series with conflicting values, which is usually a problem.