---
sort: 37
weight: 37
title: Relabeling cookbook
menu:
  docs:
    parent: 'victoriametrics'
    weight: 37
aliases:
- /relabeling.html
---

# Relabeling cookbook

VictoriaMetrics and [vmagent](https://docs.victoriametrics.com/vmagent/) support
[Prometheus-compatible relabeling](https://docs.victoriametrics.com/vmagent/#relabeling)
with [additional enhancements](https://docs.victoriametrics.com/vmagent/#relabeling-enhancements).

The relabeling is mostly used for the following tasks:

* Dropping unneeded scrape targets during [service discovery](https://docs.victoriametrics.com/sd_configs/#prometheus-service-discovery).
  See [how to drop discovered targets](#how-to-drop-discovered-targets).
* Adding or updating static labels at scrape targets. See [how to add labels to scrape targets](#how-to-add-labels-to-scrape-targets).
* Copying target labels from another labels. See [how to copy labels in scrape targets](#how-to-copy-labels-in-scrape-targets).
* Modifying scrape urls for discovered targets. See [how to modify scrape urls in targets](#how-to-modify-scrape-urls-in-targets).
* Modifying `instance` and `job` labels. See [how to modify instance and job](#how-to-modify-instance-and-job).
* Extracting label parts into another labels. See [how to extract label parts](#how-to-extract-label-parts).
* Removing prefixes from target label names. See [how to remove prefixes from target label names](#how-to-remove-prefixes-from-target-label-names).
* Removing some labels from discovered targets. See [how to remove labels from targets](#how-to-remove-labels-from-targets).
* Dropping some metrics during scape. See [how to drop metrics during scrape](#how-to-drop-metrics-during-scrape).
* Renaming scraped metrics. See [how to rename scraped metrics](#how-to-rename-scraped-metrics).
* Adding labels to scraped metrics. See [how to add labels to scraped metrics](#how-to-add-labels-to-scraped-metrics).
* Changing label values in scraped metrics. See [how to change label values in scraped metrics](#how-to-change-label-values-in-scraped-metrics).
* Removing some labels from scraped metrics. See [how to remove labels from scraped metrics](#how-to-remove-labels-from-scraped-metrics).
* Removing some labels from metrics matching some [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors).
  See [how to remove labels from metrics subset](#how-to-remove-labels-from-metrics-subset).

See also [relabeling docs at vmagent](https://docs.victoriametrics.com/vmagent/#relabeling).


## How to remove labels from metrics subset

Sometimes it may be needed to remove labels from a subset of scraped metrics, while leaving these labels in the rest of scraped metrics.
In this case the `if` [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
can be applied to `action: labeldrop` or `action: labelkeep`.

For example, the following config drops labels with names starting from `foo_` prefix from metrics matching `a{b="c"}` series selector:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - action: labeldrop
    if: 'a{b="c"}'
    regex: "foo_.*"
```

See also:

- [how to remove labels from scraped metrics](#how-to-remove-labels-from-scraped-metrics)
- [useful tips for metric relabeling](#useful-tips-for-metric-relabeling)


## How to rename scraped metrics

Metric name is a regular label with special name - `__name__` (see [these docs](https://docs.victoriametrics.com/keyconcepts/#labels)).
So renaming of metric name is performed in the same way as changing label value.

Let's look at a few examples.

The following config renames `foo` metric to `bar` across all the scraped metrics, while leaving other metric names as is:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - if: 'foo'
    replacement: bar
    target_label: __name__
```

The following config renames metrics starting from `foo_` to metrics starting from `bar_` across all the scraped metrics. For example, `foo_count` is renamed to `bar_count`:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - source_labels: [__name__]
    regex: 'foo_(.*)'
    replacement: bar_$1
    target_label: __name__
```

The following config replaces all the `-` chars in metric names with `_` chars across all the scraped metrics. For example, `foo-bar-baz` is renamed to `foo_bar_baz`:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - source_labels: [__name__]
    action: replace_all
    regex: '-'
    replacement: '_'
    target_label: __name__
```

See also [useful tips for metric relabeling](#useful-tips-for-metric-relabeling).


## How to add labels to scraped metrics

The following config sets `foo="bar"` [label](https://docs.victoriametrics.com/keyconcepts/#labels) across all the scraped metrics:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - target_label: foo
    replacement: bar
```

The following config sets `foo="bar"` label only for metrics matching `{job=~"my-app-.*",env!="dev"}` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering):

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - if: '{job=~"my-app-.*",env!="dev"}'
    target_label: foo
    replacement: bar
```

See also [useful tips for metric relabeling](#useful-tips-for-metric-relabeling).


## How to change label values in scraped metrics

The following config adds `foo_` prefix to all the values of `job` label across all the scraped metrics:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - source_labels: [job]
    target_label: job
    replacement: foo_$1
```

The following config adds `foo_` prefix to `job` label values only for metrics
matching `{job=~"my-app-.*",env!="dev"}` [series selector](https://docs.victoriametrics.com/keyconcepts/#filtering):

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - if: '{job=~"my-app-.*",env!="dev"}'
    source_labels: [job]
    target_label: job
    replacement: foo_$1
```

See also [useful tips for metric relabeling](#useful-tips-for-metric-relabeling).


## How to remove labels from scraped metrics

Sometimes it may be needed to remove labels from scraped metrics. For example, if some labels
lead to [high cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality)
or [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) issues,
then it may be a good idea to drop these labels during scrapes.
This can be done with `action: labeldrop` or `action: labelkeep` relabeling rules at `metric_relabel_configs` section:

* `action: labeldrop` drops labels with names matching the given `regex` option
* `action: labelkeep` drops labels with names not matching the given `regex` option

For example, the following config drops labels with names starting with `foo_` prefix from all the metrics scraped from the `http://host123/metrics`:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - action: labeldrop
    regex: "foo_.*"
```

The `regex` option can contain arbitrary regular expression in [RE2 format](https://github.com/google/re2/wiki/Syntax).
The `regex` option is applied to every label name in the target. It is automatically anchored, so it must match the whole label name.
The label name is left as is if the `regex` doesn't match it.

Important notes:

* Labels with `__` prefix are automatically removed after the relabeling, so there is no need in removing them with relabeling rules.
* Make sure that metrics exposed by the target can be uniquely identified by their names
  and the remaining labels after label removal. Otherwise, duplicate metrics with duplicate timestamps
  and different values will be pushed to the storage. This is an undesired issue in most cases.

See also [useful tips for metric relabeling](#useful-tips-for-metric-relabeling).


## How to drop metrics during scrape

Sometimes it is needed to drop some metrics during scrapes. For example, if some metrics result
in [high cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality)
or [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) issues,
then it may be a good idea to drop these metrics during scrapes. This can be done with the `action: drop` or `action: keep`
relabeling rules at `metric_relabel_configs` section:

* `action: drop` drops all the metrics, which match the `if` [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
* `action: keep` drops all the metrics, which don't match the `if` [series selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)

For example, the following config drops all the metrics obtained from `http://host123/metrics`, which start with `foo_` prefix:

```yaml
scrape_configs:
- job_name: test
  static_configs:
  - targets: [host123]
  metric_relabel_configs:
  - if: '{__name__=~"foo_.*"}'
    action: drop
```

Note that the relabeling config is specified under `metric_relabel_configs` section instead of `relabel_configs` section:

* The `relabel_configs` is applied to the configured/discovered targets.
* The `metric_relabel_configs` is applied to metrics scraped from the configured/discovered targets.

See also [useful tips for metric relabeling](#useful-tips-for-metric-relabeling).


## How to remove labels from a subset of targets

Sometimes it is needed to remove some labels from a subset of [discovered targets](https://docs.victoriametrics.com/sd_configs/),
while leaving these labels in the rest of discovered targets.
In this case the `if` [selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors)
can be added to `action: labeldrop` or `action: labelkeep` relabeling rule.

For example, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs),
[extracts pod-level labels](#how-to-remove-prefixes-from-target-label-names) into labels with `foo_` prefix and then drops all the labels
with `foo_bar_` prefix in their names for targets matching `{__address__=~"pod123.+"}` selector:

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

See also [how to remove labels from targets](#how-to-remove-labels-from-targets).


## How to remove labels from targets

Sometimes it is needed to remove some labels from [discovered targets](https://docs.victoriametrics.com/sd_configs/).
In this case the `action: labeldrop` and `action: labelkeep` relabeling options can be used:

* `action: labeldrop` drops all the labels with names matching the `regex` option
* `action: labelkeep` drops all the labels with names not matching the `regex` option

For example, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs),
[extracts pod-level labels](#how-to-remove-prefixes-from-target-label-names) into labels with `foo_` prefix and then drops all the labels
with `foo_bar_` prefix in their names:

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

The `regex` option can contain arbitrary regular expression in [RE2 format](https://github.com/google/re2/wiki/Syntax).
The `regex` option is applied to every label name in the target. It is automatically anchored, so it must match the whole label name.
The label name is left as is if the `regex` doesn't match it.

Important notes:

* Labels with `__` prefix are automatically removed after the relabeling, so there is no need in removing them with relabeling rules.
* Do not remove `instance` and `job` labels, since this may result in duplicate scrape targets with identical sets of labels.

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to remove prefixes from target label names

Sometimes it is needed to remove `__meta_*` prefixes from meta-labels of the [discovered targets](https://docs.victoriametrics.com/sd_configs/).
For example, [Kubernetes service discovery](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) adds `__meta_kubernetes_pod_label_<labelname>`
labels per each pod-level label. In this case it may be needed to leave only the `<labelname>` part of such label names,
while removing the `__meta_kubernetes_pod_label_` prefix. This can be done with `action: labelmap` relabeling option:

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

The `regex` option can contain arbitrary regular expression in [RE2 format](https://github.com/google/re2/wiki/Syntax).
The `regex` option is applied to every label name in the target. It is automatically anchored, so it must match the whole label name.
It can contain capture groups such as `(.+)` in the config above. These capture groups can be referenced then inside `replacement` option
with the `$N` syntax, where `N` is the number of the capture group in `regex`. The first capture group has the `$1` reference.

The label name is left as is if the `regex` doesn't match it.

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to extract label parts

Relabeling allows extracting parts from label values and storing them into arbitrary labels.
This is performed with `regex` and `replacement` options in relabeling rules.

For example, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs),
extracts `bar` part from `foo/bar` container name and stores it into the `xyz` label with `abc_` prefix:

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

The `regex` option can contain arbitrary regular expression in [RE2 format](https://github.com/google/re2/wiki/Syntax).
The `regex` option is automatically anchored, so it must match the whole value from `source_labels`.
It can contain capture groups such as `(.+)` in the config above. These capture groups can be referenced then inside `replacement` option
with the `$N` syntax, where `N` is the number of the capture group in `regex`. The first capture group has the `$1` reference.

It is possible to construct a label from multiple parts of different labels. In this case just specify the needed source labels inside `source_labels` list.
The values of labels specified in `source_labels` list are joined with `;` separator by default before being matched against the `regex`.
The separator can be overridden via `separator` option.

If the `regex` doesn't match the value constructed from `source_labels`, then the relabeling rule is skipped and the remaining relabeling rules are executed.

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to modify instance and job

Single-node VictoriaMetrics and [vmagent](https://docs.victoriametrics.com/vmagent/) automatically add `instance` and `job` labels per each discovered target:

* The `job` label is set to `job_name` value specified in the corresponding [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs).
* The `instance` label is set to the `host:port` part of `__address__` label value after target-level relabeling.
  The `__address__` label value is automatically set to the most suitable value depending
  on the used [service discovery type](https://docs.victoriametrics.com/sd_configs/#supported-service-discovery-configs).
  The `__address__` label can be overridden during relabeling - see [these docs](#how-to-modify-scrape-urls-in-targets).

Both `instance` and `job` labels can be overridden during relabeling. For example, the following config discovers pod targets
in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) and overrides `job` label from `k8s` to `foo`:

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - target_label: job
    replacement: foo
```

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to modify scrape urls in targets

URLs for scrape targets are composed of the following parts:

* Scheme (e.g. `http` or `https`). The scheme is available during target relabeling in a special label - `__scheme__`.
  By default, the scheme is set to `http`. It can be overridden either by specifying the `scheme` option
  at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs) level
  or by updating the `__scheme__` label during relabeling.
* Host and port (e.g. `host12:3456`). This information is available during target relabeling in a special label - `__address__`.
  Every [supported service discovery type](https://docs.victoriametrics.com/sd_configs/#supported-service-discovery-configs)
  sets the `__address__` label to the most suitable value. Sometimes this value needs to be modified. In this case
  just update the `__address__` label during relabeling to the needed value.
  The port part is optional. If it is missing, then it is automatically set either to `80` or `443` depending
  on the used scheme (`http` or `https`).
  The `host:port` part from the final `__address__` label is automatically set to `instance` label unless the `instance`
  label is explicitly set during relabeling.
  The `__address__` label can contain the full scrape url, e.g. `http://host:port/metrics/path?query_args`.
  In this case the `__scheme__` and `__metrics_path__` labels are ignored.
* URL path (e.g. `/metrics`). This information is available during target relabeling in a special label - `__metrics_path__`.
  By default, the `__metrics_path__` is set to `/metrics`. It can be overridden either by specifying the `metrics_path`
  option at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs)
  or by updating the `__metrics_path__` label during relabeling.
* Query args (e.g. `?foo=bar&baz=xyz`). This information is available during target relabeling in special labels
  with `__param_` prefix. For example, `__param_foo` would have the `bar` value, while `__param_baz` would have the `xyz` value
  for `?foo=bar&baz=xyz` query string. The query args can be specified either via `params` section
  at [scrape_config](https://docs.victoriametrics.com/sd_configs/#scrape_configs)
  or by updating/setting the corresponding `__param_*` labels during relabeling.

The resulting scrape url looks like the following:

```
  <__scheme__> + "://" + <__address__> + <__metrics_path__> + <"?" + query_args_from_param_labels>
```

It is expected that the target exposes metrics
in [Prometheus text exposition format](https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format)
at the resulting scrape url.

Given the scrape url construction rules above, the following config discovers pod targets
in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs)
and constructs per-target scrape url as `https://<pod_name>/foo/bar?baz=<container_name>`:

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

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to copy labels in scrape targets

Labels can be copied by specifying the source labels via `source_labels` relabeling option
and specifying the target label via `target_label` relabeling option.
For example, the following config copies `__meta_kubernetes_pod_name` label to `pod` label
for all the discovered pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - source_labels: [__meta_kubernetes_pod_name]
    target_label: pod
```

Note that the `source_labels` option accepts a list of labels in square brackets. If multiple labels are specified
in the `source_labels` list, then the specified label values are joined into a single string with `;` delimiter by default.
The delimiter can be modified by specifying it via `separator` option.
For example, the following config sets the `pod_name:container_port` value to the `host_port` label
for all the discovered pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

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

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## How to add labels to scrape targets

Additional labels can be added to scrape targets by specifying the label name in `target_label` relabeling option
and by specifying the label value in `replacement` relabeling option.
The same approach can be used for updating already existing label values at target level.

For example, the following config adds `{foo="bar"}` label to all the discovered pods in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
- job_name: k8s
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - target_label: "foo"
    replacement: "bar"
```

The labels, which are added to the target, are automatically added to all the metrics scraped from the target.
For example, if the target exposes the metric `metric{label="value"}`, then the metric is transformed into `metric{label="value",foo="bar"}`
before being sent to the storage.

If the metric exported by the target contains the same label as the target itself, then the `exported_` prefix is added to the exported label name.
For example, if the target exposes the metric `metric{foo="baz"}`, then the metric is transformed into `metric{exported_foo="baz",foo="bar"}`.
This behaviour can be changed by specifying `honor_labels: true` option at the given scrape config. In this case the exported label overrides
the target's label. In this case the `metric{foo="baz"}` stays the same. Example config with `honor_labels: true`:

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

If a particular discovered target shouldn't be scraped, then `action: keep` or `action: drop` relabeling rules
must be used inside `relabel_configs` section.

The `action: keep` keeps only scrape targets with labels matching the `if` [selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors),
while dropping the rest of targets. For example, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs)
and scrapes only pods with names starting with `foo` prefix:

```yaml
scrape_configs:
- job_name: foo_pods
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - if: '{__meta_kubernetes_pod_name=~"foo.*"}'
    action: keep
```

The `action: drop` drops all the scrape targets with labels matching the `if` [selector](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors),
while keeping the rest of targets. For example, the following config discovers pod targets in [Kubernetes](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs)
and scrapes only pods with names starting with prefixes other than `foo`:

```yaml
scrape_configs:
- job_name: not_foo_pods
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:
  - if: '{__meta_kubernetes_pod_name=~"foo.*"}'
    action: drop
```

See also [useful tips for target relabeling](#useful-tips-for-target-relabeling).


## Useful tips for target relabeling

* Target relabeling can be debugged by clicking the `debug` link for the needed target on the `http://vmagent:8429/target`
  or on the `http://vmagent:8429/service-discovery` pages. See [these docs](https://docs.victoriametrics.com/vmagent/#relabel-debug).
* Every discovered target contains a set of meta-labels, which start with `__meta_` prefix.
  The specific sets of labels per each supported service discovery option are listed
  [here](https://docs.victoriametrics.com/sd_configs/#prometheus-service-discovery).
* Every discovered target contains additional labels with `__` prefix other than `__meta_` labels.
  See [these docs](#how-to-modify-scrape-urls-in-targets) for more details.
* All the labels, which start with `__` prefix, are automatically removed from targets after the relabeling.
  So it is common practice to store temporary labels with names starting with `__` during target relabeling.
* All the target-level labels are automatically added to all the metrics scraped from targets.
* The list of discovered scrape targets with all the discovered meta-labels is available at `http://vmagent:8429/service-discovery` page for `vmagent`
  and at `http://victoriametrics:8428/service-discovery` page for single-node VictoriaMetrics.
* The list of active targets with the final set of labels left after relabeling is available at `http://vmagent:8429/targets` page for `vmagent`
  and at `http://victoriametrics:8428/targets` page for single-node VictoriaMetrics.


## Useful tips for metric relabeling

* Metric relabeling can be debugged at `http://vmagent:8429/metric-relabel-debug` page.
  See [these docs](https://docs.victoriametrics.com/vmagent/#relabel-debug).
* All the labels, which start with `__` prefix, are automatically removed from metrics after the relabeling.
  So it is common practice to store temporary labels with names starting with `__` during metrics relabeling.
* All the target-level labels are automatically added to all the metrics scraped from targets,
  so target-level labels are available during metrics relabeling.
