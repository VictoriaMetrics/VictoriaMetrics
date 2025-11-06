---
weight: 38
title: Relabeling cookbook
menu:
  docs:
    parent: "victoriametrics"
    weight: 38
tags:
  - metrics
aliases:
  - /relabeling.html
  - /relabeling/index.html
  - /relabeling/
---

The relabeling cookbook provides practical examples and patterns for
transforming your metrics data as it flows through VictoriaMetrics, helping you
control what gets collected and how it's labeled.

VictoriaMetrics and vmagent support Prometheus-style relabeling with
[extra features](#relabeling-enhancements) to enhance the functionality.

The following articles contain useful information about Prometheus relabeling:

- [How to use Relabeling in Prometheus and VictoriaMetrics](https://valyala.medium.com/how-to-use-relabeling-in-prometheus-and-victoriametrics-8b90fc22c4b2)

## Relabeling Stages

Relabeling in VictoriaMetrics happens in three main stages:

### Service Discovery Relabeling

Relabeling starts with `relabel_configs` in the prometheus scrape config
(`-promscrape.config`).

```yaml {hl_lines=[3,8]}
# global relabeling rules applied to all targets
global:
  relabel_configs:

# job-specific target relabeling applied only to targets within this job
scrape_configs:
  - job_name: "my-job"
    relabel_configs:
```

These rules are used during service discovery, before VictoriaMetrics begins
scraping metrics from targets.

- `global.relabel_configs`: applied to all discovered targets from all jobs.
- `scrape_configs[].relabel_configs`: applied only to targets within the
  specified job.

The main purpose is to change or filter the list of discovered targets. You can
add, remove, or update target labels—or drop targets completely.

Refer to the
[Service Discovery Relabeling Cheatsheet](#service-discovery-relabeling-cheatsheet)
section for more examples.

### Scraping Relabeling

Once VictoriaMetrics has finished selecting the targets using `relabel_configs`,
it starts scraping those endpoints. After scraping, you can apply
`metric_relabel_configs` in the `-promscrape.config` file
{{% available_from "v1.106.0" %}} to modify the scraped metrics:

```yaml {hl_lines=[3,8]}
# global metric relabeling applied to all metrics
global:
  metric_relabel_configs:

# scrape relabeling applied to all metrics
scrape_configs:
  - job_name: "my-job"
    metric_relabel_configs:
```

This is the second stage, and it operates on **individual metrics** that were
just scraped from the targets, not the targets themselves.

- `global.metric_relabel_configs`: affects all scraped metrics from all jobs.
- `scrape_configs[].metric_relabel_configs`: applies only to metrics scraped
  from the specific job.

This means you can filter or modify the scraped time series before
VictoriaMetrics stores them in its time series database.

Refer to the [Scraping Relabeling Cheatsheet](#scraping-relabeling-cheatsheet)
section for more examples.

### Remote Write Relabeling

This step takes place after `metric_relabel_configs` are applied, right before
metrics are sent to a storage destination specified by `remoteWrite.url` in
`vmagent`.

The main goal of this stage is to apply relabeling rules to all incoming
metrics, no matter where they come from (push-based or pull-based sources). It
includes two phases:

- `-remoteWrite.relabelConfig`: This is applied to all metrics before they are sent to any remote storage destination.
  Config content is available at `http://vmagent-host:8429/remotewrite-relabel-config` endpoint {{% available_from "v1.129.0" %}}. 
- `-remoteWrite.urlRelabelConfig`: This is applied to all metrics before they are sent to a specific remote storage destination.
  Config content is available at `http://vmagent-host:8429/remotewrite-url-relabel-config` endpoint {{% available_from "v1.129.0" %}}.

This functionality is essential for routing and filtering data in different ways
for multiple backends. For example:

- Send only metrics with `env=prod` to a production VictoriaMetrics cluster (see
  [Splitting data streams among multiple systems](https://docs.victoriametrics.com/victoriametrics/vmagent/#splitting-data-streams-among-multiple-systems)
  for how to configure this in `vmagent`).
- Send only metrics with `env=dev` to a development cluster.
- Send a subset of high-importance metrics to a Kafka topic for real-time
  analysis, while sending all metrics to long-term storage.
- Remove certain labels only for a specific backend, while keeping them for
  others.

## Relabeling Enhancements

VictoriaMetrics relabeling is compatible with
[Prometheus relabeling](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config)
and provides the following enhancements:

- **The `replacement` field**: allows you to construct new label values
  by referencing existing ones using the `{{label_name}}` syntax.
  For example, if a metric has the labels
  `{instance="host123", job="node_exporter"}`,
  this rule will create or update the `fullname` label with the value
  `host123-node_exporter`:

  ```yaml {hl_lines=[2]}
  - target_label: "fullname"
    replacement: "{{instance}}-{{job}}"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+%22fullname%22%0A++replacement%3A+%22%7B%7Binstance%7D%7D-%7B%7Bjob%7D%7D%22&labels=%7B__name__%3D%22node_cpu_seconds_total%22%2C+instance%3D%22server-1%3A9100%22%2C+job%3D%22node_exporter%22%2C+mode%3D%22idle%22%2C+cpu%3D%220%22%7D)

- **The `if` filter**: applies the `action` only to samples that match one or
  more
  [time series selectors](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering).
  It supports a single selector or a list. If any selector matches, the `action`
  is applied.

  For example, the following relabeling rule keeps metrics matching
  `node_memory_MemAvailable_bytes{instance="host123"}` series selector, while
  dropping the rest of metrics:

  ```yaml {hl_lines=[1]}
  - if: 'node_memory_MemAvailable_bytes{instance="host123"}'
    action: keep
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27node_memory_MemAvailable_bytes%7Binstance%3D%22host123%22%7D%27%0A++action%3A+keep&labels=%7B__name__%3D%22node_memory_MemAvailable_bytes%22%2C+instance%3D%22host456%22%2C+job%3D%22node_exporter%22%7D)

  This is equivalent to the following, less intuitive Prometheus-compatible
  rule:

  ```yaml
  - action: keep
    source_labels: [__name__, instance]
    regex: "node_memory_MemAvailable_bytes;host123"
  ```

  The `if` option can include multiple filters. If any one of them matches a
  sample, the action will be applied. For example, the rule below adds the label
  `team="infra"` to all samples where `job="api"` OR `instance="web-1"`:

  ```yaml {hl_lines=[3]}
  - target_label: team
    replacement: infra
    if:
      - '{job="api"}'
      - '{instance="web-1"}'
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+team%0A++replacement%3A+infra%0A++if%3A%0A++-+%27%7Bjob%3D%22api%22%7D%27%0A++-+%27%7Binstance%3D%22web-1%22%7D%27&labels=%7B__name__%3D%22http_requests_total%22%2C+job%3D%22api%22%2C+instance%3D%22web-2%22%7D)

- **The `regex`**: can be split into multiple lines for better readability.
  VictoriaMetrics automatically combines them using `|` (OR). The two examples
  below are treated the same and match `http_requests_total`,
  `node_memory_MemAvailable_bytes`, or any metric starting with `nginx_`:

  ```yaml {hl_lines=[2]}
  - action: keep_metrics
    regex: "http_requests_total|node_memory_MemAvailable_bytes|nginx_.+"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+keep_metrics%0A++regex%3A+%22http_requests_total%7Cnode_memory_MemAvailable_bytes%7Cnginx_.%2B%22&labels=%7B__name__%3D%22nginx_latency_seconds%22%2C+instance%3D%22host2%22%7D)

  ```yaml {hl_lines=[2]}
  - action: keep_metrics
    regex:
      - "http_requests_total"
      - "node_memory_MemAvailable_bytes"
      - "nginx_.+"
  ```

Beside enhancements, VictoriaMetrics also provides the following new actions:

- **`replace_all` action**: Replaces all matches of regex in `source_labels`
  with replacement, and writes the result to `target_label`. Example: replaces
  all dashes `-` with underscores `_`in metric names
  (e.g.`http-request-latency`to`http_request_latency`):

  ```yaml {hl_lines=[1]}
  - action: replace_all
    source_labels: ["__name__"]
    target_label: "__name__"
    regex: "-"
    replacement: "_"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+replace_all%0A++source_labels%3A+%5B%22__name__%22%5D%0A++target_label%3A+%22__name__%22%0A++regex%3A+%22-%22%0A++replacement%3A+%22_%22&labels=%7B__name__%3D%22http-request-latency%22%2C+instance%3D%22server-1%22%7D)

- **`labelmap_all` action**: allows you to create new labels by renaming
  existing ones based on a regex pattern match against the original label's
  name. Example: Replace `-` with `_` in all label names (e.g.
  `pod-label-region` → `pod_label_region`):

  ```yaml {hl_lines=[1]}
  - action: labelmap_all
    regex: "-"
    replacement: "_"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labelmap_all%0A++regex%3A+%22-%22%0A++replacement%3A+%22_%22&labels=%7B__name__%3D%22http_requests_total%22%2C+pod-label-region%3D%22us-west%22%2C+pod-label-app%3D%22frontend%22%7D)

- **`keep_if_equal` action**: Keeps the entry only if all `source_labels` have
  the same value. Example: Keep targets where `instance` and `host` are equal:

  ```yaml {hl_lines=[1]}
  - action: keep_if_equal
    source_labels: ["instance", "host"]
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+keep_if_equal%0A++source_labels%3A+%5B%22instance%22%2C+%22host%22%5D&labels=%7B__name__%3D%22node_cpu_seconds_total%22%2C+instance%3D%22srv2%22%2C+host%3D%22srv3%22%7D)

- **`drop_if_equal` action**: Drops the entry if all `source_labels` have the
  same value. Example: Drop targets where `instance` equals `host`:

  ```yaml {hl_lines=[1]}
  - action: drop_if_equal
    source_labels: ["instance", "host"]
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+drop_if_equal%0A++source_labels%3A+%5B%22instance%22%2C+%22host%22%5D&labels=%7B__name__%3D%22node_cpu_seconds_total%22%2C+instance%3D%22srv3%22%2C+host%3D%22srv3%22%7D)

- **`keep_if_contains` action**: Keeps the entry if `target_label` contains all
  values from `source_labels`. Example: Keep if `__meta_consul_tags` contains
  the value of `required_tag`:

  ```yaml {hl_lines=[1]}
  - action: keep_if_contains
    target_label: __meta_consul_tags
    source_labels: [required_tag]
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+keep_if_contains%0A++target_label%3A+__meta_consul_tags%0A++source_labels%3A+%5Brequired_tag%5D&labels=%7B__name__%3D%22up%22%2C+__meta_consul_tags%3D%22dev%2Capi%22%2C+required_tag%3D%22api%22%7D)

- **`drop_if_contains` action**: Drops the entry if `target_label` contains all
  values from `source_labels`. Example: Drop if `__meta_consul_tags` label value
  contains the value of `blocked_tag` label value:

  ```yaml {hl_lines=[1]}
  - action: drop_if_contains
    target_label: __meta_consul_tags
    source_labels: [blocked_tag]
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+drop_if_contains%0A++target_label%3A+__meta_consul_tags%0A++source_labels%3A+%5Bblocked_tag%5D%0A&labels=%7B__name__%3D%22up%22%2C+__meta_consul_tags%3D%22prod%2Capi%22%2C+blocked_tag%3D%22api%22%7D)

- **`keep_metrics` action**: Keeps metrics whose names match the `regex`.
  Example: Keep only `http_requests_total` and `node_memory_Active_bytes`
  metrics:

  ```yaml {hl_lines=[1]}
  - action: keep_metrics
    regex: "http_requests_total|node_memory_Active_bytes"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+keep_metrics%0A++regex%3A+%22http_requests_total%7Cnode_memory_Active_bytes%22&labels=%7B__name__%3D%22http_requests_total%22%2C+job%3D%22api%22%7D)

- **`drop_metrics` action**: Drops metrics whose names match the `regex`.
  Example: Drop `go_gc_duration_seconds` and `process_cpu_seconds_total`
  metrics:

  ```yaml {hl_lines=[1]}
  - action: drop_metrics
    regex: "go_gc_duration_seconds|process_cpu_seconds_total"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+drop_metrics%0A++regex%3A+%22go_gc_duration_seconds%7Cprocess_cpu_seconds_total%22&labels=%7B__name__%3D%22go_gc_duration_seconds%22%2C+job%3D%22go-app%22%7D)

- **`graphite` action**: Applies Graphite-style relabeling rules to extract
  labels from metric names
  ([Try it](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+graphite%0A++match%3A+%27*.server.*.total%27%0A++labels%3A%0A++++__name__%3A+%27%24%7B2%7D_total%27%0A++++instance%3A+%27%24%7B2%7D%3A9100%27%0A++++job%3A+%27%241%27&labels=%7B__name__%3D%22app1.server.requests.total%22%7D)).
  See [Graphite Relabeling](#graphite-relabeling) for details.

### Graphite Relabeling

VictoriaMetrics components support `action: graphite` relabeling rules. These
rules let you pull parts from Graphite-style metric names and turn them into
Prometheus labels. (The matching syntax is similar to
[Glob matching in statsd_exporter](https://github.com/prometheus/statsd_exporter#glob-matching))

You must set the `__name__` label inside the `labels` section to define the new
metric name. Otherwise, the original metric name remains unchanged.

For example, this rule transforms a Graphite-style metric like
`authservice.us-west-2.login.total` into a Prometheus-style metric
`login_total{instance="us-west-2:8080", job="authservice"}`:

```yaml {hl_lines=[1]}
- action: graphite
  match: "*.*.*.total"
  labels:
    __name__: "${3}_total"
    job: "$1"
    instance: "${2}:8080"
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+graphite%0A++match%3A+%22*.*.*.total%22%0A++labels%3A%0A++++__name__%3A+%22%24%7B3%7D_total%22%0A++++job%3A+%22%241%22%0A++++instance%3A+%22%24%7B2%7D%3A8080%22&labels=%7B__name__%3D%22authservice.us-west-2.login.total%22%7D)

Key points about `action: graphite` relabeling:

- The rule applies only to metrics that match the `match` pattern. Others are
  ignored.
- `*` matches as many characters as possible until the next `.` or next match
  part. It can also match nothing if followed by a dot. E.g.,
  `match: "app*prod.requests"` matches `app42prod.requests`, and `42` becomes
  available as `$1` in the `labels` section.
- `$0` is the full original metric name.
- Rules run in the order they appear in the config.

Using `action: graphite` is typically easier and faster than using
`action: replace` for parsing Graphite-style metric names.

## Relabel Debugging

`vmagent` and single-node VictoriaMetrics support debugging at both the target
and metric levels.

Start by visiting `http://vmagent:8429/targets` for vmagent or
`http://victoriametrics:8428/targets` for single-node VictoriaMetrics. You will
see two types of targets:

- Active Targets (`/targets`): These are the targets that vmagent is currently
  scraping. Target relabeling rules have already been applied.
- Discovered Targets (`/service-discovery`): These are the targets found during
  service discovery, before any relabeling rules are applied. This includes
  targets that may later be dropped.

_This option is only available when the component is started with the
`-promscrape.dropOriginalLabels=false` flag._

{{% collapse name="How to use `/targets` page?" %}}

This `/targets` page helps answer the following questions:

**1. Why are some targets not being scraped?**

- The `last error` column shows the reason why a target is not being scraped.
- Click the `endpoint` link to open the target URL in your browser.
- Click the `response` link to view the response vmagent received from the
  target.

**2. What labels does a specific target have?**

The `labels` column shows the labels for each target. These labels are attached
to all metrics scraped from that target.

You can click the label column of the target to see the original labels
**before** any relabeling was applied.

_This option is only available when the component is started with the
`-promscrape.dropOriginalLabels=false` flag._

**3. Why does a target have a certain set of labels?**

Click the `target` link in the `debug relabeling` column. This opens a
step-by-step view of how the relabeling rules were applied to the original
labels.

_This option is only available when the component is started with the
`-promscrape.dropOriginalLabels=false` flag._

**4. How are metric relabeling rules applied to scraped metrics?**

Click the `metrics` link in the `debug relabeling` column. This shows how the
metric relabeling rules were applied, step by step.

Each column on the page shows important details:

- `state`: shows if the target is currently up or down.
- `scrapes`: number of times the target was scraped.
- `errors`: number of failed scrapes.
- `last scrape`: when the last scrape happened.
- `last scrape size`: size of the last scrape.
- `duration`: time taken for the last scrape.
- `samples`: number of metrics exposed by the target during the last scrape.

{{% /collapse %}}

{{% collapse name="How to use `/service-discovery` page?" %}}

This page shows all
[discovered targets](https://docs.victoriametrics.com/victoriametrics/sd_configs/).

_This option is only available when the component is started with the
`-promscrape.dropOriginalLabels=false` flag._

It helps answer the following questions:

**1. Why are some targets dropped during service discovery or showing unexpected
labels?**

Click the `debug` link in the `debug relabeling` column for a dropped target.
This opens a step-by-step view of how
[target relabeling rules](#relabeling-stages) were applied to that target's
original labels.

**2. What were the original labels before relabeling?**

The `discovered labels` column shows the original labels for each discovered
target.

{{% /collapse %}}

## Relabeling Use Cases

### Service Discovery Relabeling Cheatsheet

**Target-level relabeling** is applied during
[service discovery](https://docs.victoriametrics.com/victoriametrics/sd_configs/#prometheus-service-discovery)
and affects the targets (which will be scraped), their labels and all the
metrics scraped from them:

{{% collapse name="Remove discovered targets" %}}

#### How to drop discovered targets

To drop a particular discovered target, use the following options:

- `action: drop`: drops scrape targets with labels matching the `if`
  [series selector](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering)
- `action: keep`: keeps scrape targets with labels matching the `if` series
  selector, while dropping all other targets

Here are examples of these options:

- This config discovers pods in
  [Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs)
  and drops all pods with names starting with the `test-` prefix:

  ```yaml
  scrape_configs:
    - job_name: prod_pods_only
      kubernetes_sd_configs:
        - role: pod
      relabel_configs:
        - if: '{__meta_kubernetes_pod_name=~"test-.*"}'
          action: drop
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27%7B__meta_kubernetes_pod_name%3D%7E%22test-.*%22%7D%27%0A++action%3A+drop&labels=%7B__meta_kubernetes_pod_name%3D%22test-payment-7cbd8d77b6-4l5xv%22%2Cnamespace%3D%22qa%22%2Capp%3D%22payment-service%22%7D)

- This config keeps only pods with names starting with the `backend-` prefix:

  ```yaml
  scrape_configs:
    - job_name: backend_pods
      kubernetes_sd_configs:
        - role: pod
      relabel_configs:
        - if: '{__meta_kubernetes_pod_name=~"backend-.*"}'
          action: keep
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27%7B__meta_kubernetes_pod_name%3D%7E%22backend-.*%22%7D%27%0A++action%3A+keep&labels=%7B__meta_kubernetes_pod_name%3D%22frontend-auth-5cbdbb7ff8-qf82n%22%2Cnamespace%3D%22prod%22%2Ccontainer%3D%22auth%22%7D)

See also
[useful tips for target relabeling](#useful-tips-for-target-relabeling).

{{% /collapse %}}

{{% collapse name="Change which URL is used to fetch metrics from targets" %}}

#### How to modify scrape URLs in targets

URLs for scrape targets are composed of the following parts:

- Scheme (e.g. `http`, `https`) is available during target relabeling in a
  special label - `__scheme__`. By default, it's set to `http` but can be
  overridden either by specifying the `scheme` option at
  [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs)
  level or by updating the `__scheme__` label during relabeling.
- Host and port (e.g. `host12:3456`) is available during target relabeling in a
  special label - `__address__`. Its value depends on the
  [service discovery type](https://docs.victoriametrics.com/victoriametrics/sd_configs/#supported-service-discovery-configs).
  Sometimes this value needs to be modified. In this case, just update the
  `__address__` label during relabeling to the needed value.
  - The port part is optional. If it is missing, it's automatically set
    depending on the scheme (`80` for `http` or `443` for `https`). The
    `host:port` part from the final `__address__` label is automatically set to
    the `instance` label. The `__address__` label can contain the full scrape
    URL (e.g. `http://host:port/metrics/path?query_args`). In this case the
    `__scheme__` and `__metrics_path__` labels are ignored.
- URL path (e.g. `/metrics`) is available during target relabeling in a special
  label - `__metrics_path__`. By default, it's set to `/metrics` and can be
  overridden either by specifying the `metrics_path` option at
  [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs)
  level or by updating the `__metrics_path__` label during relabeling.
- Query args (e.g. `?foo=bar&baz=xyz`) are available during target relabeling in
  special labels with the `__param_` prefix.
  - Take `?foo=bar&baz=xyz` for example. There will be two special labels:
    `__param_foo="bar"` and `__param_baz="xyz"`. The query args can be specified
    either via the `params` section at
    [scrape_config](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs)
    or by updating/setting the corresponding `__param_*` labels during
    relabeling.

The resulting scrape URL looks like the following:

```go
<__scheme__> + "://" + <__address__> + <__metrics_path__> + <"?" + query_args_from_param_labels>
```

Given the scrape URL construction rules above, the following config discovers
pod targets in
[Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs)
and constructs a per-target scrape URL as
`https://<pod_name>/metrics/container?name=<container_name>`:

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    metrics_path: /metrics/container
    relabel_configs:
      - target_label: __scheme__
        replacement: https
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: __address__
      - source_labels: [__meta_kubernetes_pod_container_name]
        target_label: __param_name
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+__scheme__%0A++replacement%3A+https%0A-+source_labels%3A+%5B__meta_kubernetes_pod_name%5D%0A++target_label%3A+__address__%0A-+source_labels%3A+%5B__meta_kubernetes_pod_container_name%5D%0A++target_label%3A+__param_name&labels=%7B__meta_kubernetes_pod_name%3D%22checkout-api-58c9d%22%2C+__meta_kubernetes_pod_container_name%3D%22app%22%2C+__meta_kubernetes_namespace%3D%22production%22%2C+__meta_kubernetes_pod_ip%3D%2210.42.6.25%22%2C+__address__%3D%2210.42.6.25%3A9100%22%2C+job%3D%22k8s%22%2C+instance%3D%2210.42.6.25%3A9100%22%7D)

{{% /collapse %}}

{{% collapse name="Remove labels from discovered targets" %}}

#### How to remove labels from targets

To remove some labels from targets discovered by the scrape job, use either:

- `action: labeldrop`: drops labels with names matching the given `regex` option
- `action: labelkeep`: drops labels with names not matching the given `regex`
  option

For example:

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - action: labelmap
        regex: "__meta_kubernetes_pod_label_(.+)"
        replacement: "pod_label_$1"
      - action: labeldrop
        regex: "pod_label_team_.*"
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labelmap%0A++regex%3A+%22__meta_kubernetes_pod_label_%28.%2B%29%22%0A++replacement%3A+%22pod_label_%241%22%0A-+action%3A+labeldrop%0A++regex%3A+%22pod_label_team_.*%22&labels=%7Bcontainer%3D%22nginx%22%2C+namespace%3D%22default%22%2C+__meta_kubernetes_pod_label_app_kubernetes_io_name%3D%22nginx%22%2C+__meta_kubernetes_pod_label_team_backend%3D%22infra%22%2C+__meta_kubernetes_pod_label_team_frontend%3D%22dashboard%22%2C+__meta_kubernetes_pod_label_env%3D%22production%22%7D)

The job above will:

1. discover pods in
   [Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs)
2. extract pod-level labels (e.g. `app.kubernetes.io/name` and `team`)
3. prefix them with `pod_label_` and add them as labels to all scraped metrics
4. drop all labels starting with `pod_label_team_`
5. drop all labels starting with `__` (this is done by default by
   VictoriaMetrics)

Note that:

- Labels that start with `__` are removed automatically after relabeling, so you
  don't need to drop them with relabeling rules.
- Do not remove `instance` and `job` labels, since this may result in duplicate
  scrape targets with identical sets of labels.
- The `regex` option must match the whole label name from start to end, not just
  a part of it.

{{% /collapse %}}

{{% collapse name="Remove labels from a subset of targets" %}}

#### How to remove labels from a subset of targets

To remove some labels from a subset of discovered targets while keeping the rest
of the targets unchanged, use the `if`
[series selector](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering)
with `action: labeldrop` or `action: labelkeep` relabeling rule.

As an illustration:

- The job below discovers Kubernetes pods and removes any labels starting with
  `pod_internal_` but only for targets matching the `{__address__=~"pod123.+"}`
  selector

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - action: labeldrop
        if: '{__address__=~"pod123.+"}'
        regex: "pod_internal_.*"
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labeldrop%0A++if%3A+%27%7B__address__%3D%7E%22pod123.%2B%22%7D%27%0A++regex%3A+%22pod_internal_.*%22&labels=%7B__address__%3D%22pod123-api-0.default.svc%3A8080%22%2C+__meta_kubernetes_namespace%3D%22production%22%2C+pod_internal_cost_center%3D%22devops%22%2C+pod_internal_sensitive%3D%22true%22%2C+pod_label_app%3D%22api%22%2C+job%3D%22k8s%22%7D)

{{% /collapse %}}

{{% collapse name="Remove prefixes from label names" %}}

#### How to remove prefixes from target label names

You can modify target-labels including removing prefixes with the
`action: labelmap` option.

For example,
[Kubernetes service discovery](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs)
automatically adds special `__meta_kubernetes_pod_label_<labelname>` labels for
each pod-level label.

All labels with the prefix `__` will be dropped automatically. To extract and
keep only the `<labelname>` part of this special label, you can use
`action: labelmap` combined with `regex` and `replacement` options:

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

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labelmap%0A++regex%3A+%22__meta_kubernetes_pod_label_%28.%2B%29%22%0A++replacement%3A+%22%241%22&labels=%7B__address__%3D%2210.42.3.57%3A8080%22%2C+container%3D%22nginx%22%2C+pod%3D%22nginx-prod-5d9f%22%2C+namespace%3D%22default%22%2C+__meta_kubernetes_pod_label_app%3D%22nginx%22%2C+__meta_kubernetes_pod_label_env%3D%22production%22%2C+__meta_kubernetes_pod_label_team%3D%22devops%22%7D)

The regex contains a capture group `(.+)`. This capture group can be referenced
inside the `replacement` option with the `$N` syntax, such as `$1` for the first
capture group.

This config will create a new label with the name extracted from the regex
capture group `(.+)` for all metrics scraped from the discovered pods.

Note that:

- The `regex` option must match the whole label name from start to end, not just
  a part of it.

{{% /collapse %}}

{{% collapse name="Add or update labels by extracting values of other labels" %}}

#### How to extract label parts

Relabeling allows extracting parts from label values and storing them into
arbitrary labels. This is performed with:

- `source_labels`: the label(s) whose values are used to compute the new value
  for `target_label`,
- `target_label`: the label we want to modify or create,
- `replacement`: the value that will be computed and assigned to the
  `target_label`,
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
        replacement: "team_$1"
        target_label: owner_team
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5B__meta_kubernetes_pod_container_name%5D%0A++regex%3A+%22%5B%5E%2F%5D%2B%2F%28.%2B%29%22%0A++replacement%3A+%22team_%241%22%0A++target_label%3A+owner_team&labels=%7B__address__%3D%2210.42.3.99%3A8080%22%2Cpod%3D%22orders-backend-5489f%22%2Cnamespace%3D%22production%22%2Ccontainer%3D%22backend%22%2C__meta_kubernetes_pod_container_name%3D%22app%2Fbackend%22%7D)

The job above discovers pod targets in
[Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs),
and performs these actions:

1. Extracts the value of `__meta_kubernetes_pod_container_name` label (e.g.
   `foo/bar`),
2. Matches it against the regex `[^/]+/(.+)`,
3. Computes the new value as `team_$1` with `$1` capture from regex `(.+)`,
4. Stores the result in the `owner_team` label.

Note that:

- The `regex` option must match the whole label value from start to end, not
  just a part of it.
- If `source_labels` contains multiple labels, their values are joined with a
  `;` separator (customized by the `separator` option) before being matched
  against the `regex`.

{{% /collapse %}}

{{% collapse name="Add or update labels on discovered targets" %}}

#### How to add labels to scrape targets

To add or update labels on scrape targets during discovery, use these options:

- `target_label`: specifies the label name to add or update
- `replacement`: specifies the value to assign to this label

For example, this config adds a `environment="production"` label to all
discovered pods in
[Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - target_label: "environment"
        replacement: "production"
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+%22environment%22%0A++replacement%3A+%22production%22&labels=%7Bcontainer%3D%22redis%22%2C+namespace%3D%22cache%22%2C+pod%3D%22redis-cache-9df49c5b9-hxz6m%22%7D)

If a label from the scrape configuration (`target_label`) conflicts with a label
from the scraped metric (scrape-time label), the original scrape-time label is
renamed by adding an `exported_` prefix.

To avoid this renaming and instead let the scrape-time labels take priority
(overriding target labels), set `honor_labels: true` in the scrape
configuration.

For example, this config adds a `environment="production"` label to all
discovered pods, but if any pod already exports a `environment` label, that
value will override the target label:

```yaml {hl_lines="5"}
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    honor_labels: true
    relabel_configs:
      - target_label: "environment"
        replacement: "production"
```

See also
[useful tips for target relabeling](#useful-tips-for-target-relabeling).

{{% /collapse %}}

{{% collapse name="Add labels by copying from other labels" %}}

#### How to copy labels in scrape targets

Labels can be copied using the following options:

- `source_labels`: specifies which labels to copy from
- `target_label`: specifies the destination label to receive the value

The following config copies the `__meta_kubernetes_pod_name` label to the `pod`
label for all discovered pods in
[Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5B__meta_kubernetes_pod_name%5D%0A++target_label%3A+pod&labels=%7B__meta_kubernetes_pod_name%3D%22nginx-deployment-65f7c58c5b-bxjzs%22%2Cnamespace%3D%22default%22%2Ccontainer%3D%22nginx%22%2Cpod_name%3D%22nginx-deployment-65f7c58c5b-bxjzs%22%7D)

If `source_labels` contains multiple labels, their values are joined with a `;`
delimiter by default. Use the `separator` option to change this delimiter.

For example, this config combines pod name and container port into the
`host_port` label for all discovered pod targets in
[Kubernetes](https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs):

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels:
          [
            __meta_kubernetes_pod_name,
            __meta_kubernetes_pod_container_port_number,
          ]
        separator: ":"
        target_label: host_port
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5B__meta_kubernetes_pod_name%2C+__meta_kubernetes_pod_container_port_number%5D%0A++separator%3A+%22%3A%22%0A++target_label%3A+host_port&labels=%7B__meta_kubernetes_pod_name%3D%22api-server-746d95f76f-7r2hp%22%2C__meta_kubernetes_pod_container_port_number%3D%228080%22%2Cnamespace%3D%22backend%22%2Ccontainer%3D%22api%22%2Cinterface%3D%22eth0%22%7D)

{{% /collapse %}}

{{% collapse name="Rename `instance` and `job` labels" %}}

#### How to modify instance and job

`instance` and `job` labels are automatically added by single-node
VictoriaMetrics and
[vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) for each
discovered target.

- The `job` label is set to the `job_name` value specified in the corresponding
  `scrape_config`.
- The `instance` label is set to the `host:port` part of the `__address__` label
  value after target-level relabeling. The `__address__` label value depends on
  the type of
  [service discovery](https://docs.victoriametrics.com/victoriametrics/sd_configs/#supported-service-discovery-configs)
  and
  [can be overridden](https://docs.victoriametrics.com/victoriametrics/sd_configs/#scrape_configs)
  during relabeling.

Modifying `instance` and `job` labels works like other target-labels by using
`target_label` and `replacement` options:

```yaml
scrape_configs:
  - job_name: k8s
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - target_label: job
        replacement: kubernetes_pod_metrics
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+job%0A++replacement%3A+kubernetes_pod_metrics&labels=%7B__address__%3D%2210.42.3.99%3A8080%22%2C+container%3D%22checkout%22%2C+pod%3D%22checkout-api-5d9f%22%2C+namespace%3D%22production%22%2C+job%3D%22k8s%22%2C+instance%3D%2210.42.3.99%3A8080%22%7D)

{{% /collapse %}}

Note: All the target-level labels which are not prefixed with `__` are
automatically added to all the metrics scraped from targets.

### Scraping Relabeling Cheatsheet

**Metric-level relabeling** is applied after metrics are scraped (scraping
relabeling `metric_relabel_configs` and remote write relabeling
`-remoteWrite.urlRelabelConfig`) and affects the individual metrics:

{{% collapse name="How to remove labels from scraped metrics" %}}

#### How to remove labels from scraped metrics

Removing labels from scraped metrics is a good idea to avoid
[high cardinality](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-cardinality)
and
[high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate)
issues.

This can be done with either of the following actions:

- `action: labeldrop`: drops labels with names matching the given `regex` option
- `action: labelkeep`: drops labels with names not matching the given `regex`
  option

Let's see this in action:

- Remove labels with names starting with the `kubernetes_` prefix from all
  scraped metrics:

  ```yaml
  metric_relabel_configs:
    - action: labeldrop
      regex: "kubernetes_.*"
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labeldrop%0A++regex%3A+%22kubernetes_.*%22&labels=%7B__name__%3D%22container_cpu_usage_seconds_total%22%2Ccontainer%3D%22app%22%2C+kubernetes_namespace%3D%22default%22%2C+kubernetes_pod_name%3D%22app-123%22%7D)

The `regex` option must match the whole label name from start to end, not just a
part of it.

Note that:

- Labels that start with `__` are removed automatically after relabeling, so you
  don't need to drop them with relabeling rules.

{{% /collapse %}}

{{% collapse name="How to remove labels from metrics subset" %}}

#### How to remove labels from metrics subset

You can remove certain labels from some metrics without affecting other metrics
by using the `if` parameter with `labeldrop` action. The `if` parameter is a
[series selector](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering) -
it looks at the metric name and labels of each scraped time series.

For instance, this config below removes the `cpu` and `mode` labels, but only
from the `node_cpu_seconds_total` metric where `mode="idle"`:

```yaml
metric_relabel_configs:
  - action: labeldrop
    if: 'node_cpu_seconds_total{mode="idle"}'
    regex: "cpu|mode"
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+action%3A+labeldrop%0A++if%3A+%27node_cpu_seconds_total%7Bmode%3D%22idle%22%7D%27%0A++regex%3A+%22cpu%7Cmode%22&labels=%7B__name__%3D%22node_cpu_seconds_total%22%2C+mode%3D%22idle%22%2C+cpu%3D%220%22%2C+node%3D%22A%22%7D)

{{% /collapse %}}

{{% collapse name="How to add labels to scraped metrics" %}}

#### How to add labels to scraped metrics

You can add custom labels to scraped metrics using `target_label` to set the
label name and the `replacement` field to set the label value. For example:

- Add a `region="us-east-1"` label to all scraped metrics:

  ```yaml
  metric_relabel_configs:
    - target_label: region
      replacement: us-east-1
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+target_label%3A+region%0A++replacement%3A+us-east-1&labels=%7B__name__%3D%22node_memory_MemAvailable_bytes%22%2C+instance%3D%22server-01%3A9100%22%7D)

- Add a `team="platform"` label only for metrics from jobs that match `web-.*`
  and are not in the staging environment :

  ```yaml
  metric_relabel_configs:
    - if: '{job=~"web-.*", environment!="staging"}'
      target_label: team
      replacement: platform
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27%7Bjob%3D%7E%22web-.*%22%2C+environment%21%3D%22staging%22%7D%27%0A++target_label%3A+team%0A++replacement%3A+platform&labels=%7B__name__%3D%22http_requests_total%22%2C+job%3D%22web-api%22%2C+environment%3D%22prod%22%7D)

{{% /collapse %}}

{{% collapse name="How to change label values in scraped metrics" %}}

#### How to change label values in scraped metrics

To change the label values of scraped metrics, we use the following fields:

- `target_label`: the label we want to modify (if it exists) or create,
- `source_labels`: the label(s) whose values are used to compute the new value
  for `target_label`,
- `replacement`: the value that will be computed and assigned to the
  `target_label`.

Below are a few illustrations:

- Add `prod_` prefix to all values of the job label across all scraped metrics:

  ```yaml
  metric_relabel_configs:
    - source_labels: [job]
      target_label: job
      replacement: prod_$1
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5Bjob%5D%0A++target_label%3A+job%0A++replacement%3A+prod_%241&labels=%7B__name__%3D%22node_memory_Active_bytes%22%2C+job%3D%22node-exporter%22%2C+instance%3D%2210.0.0.1%3A9100%22%7D)

- Add `prod_` prefix to `job` label values only for metrics matching
  `{job=~"api-service-.*",env!="dev"}`:

  ```yaml
  metric_relabel_configs:
    - if: '{job=~"api-service-.*",env!="dev"}'
      source_labels: [job]
      target_label: job
      replacement: prod_$1
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27%7Bjob%3D%7E%22api-service-.*%22%2Cenv%21%3D%22dev%22%7D%27%0A++source_labels%3A+%5Bjob%5D%0A++target_label%3A+job%0A++replacement%3A+prod_%241&labels=%7B__name__%3D%22http_requests_total%22%2Cjob%3D%22api-service-orders%22%2C+env%3D%22staging%22%2C+instance%3D%2210.0.0.5%3A8080%22%7D)

{{% /collapse %}}

{{% collapse name="How to rename scraped metrics" %}}

#### How to rename scraped metrics

The metric name is actually the value of a special label called `__name__` (see
[Key Concepts](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels)).
So renaming a metric is performed in the same way as changing a label value.
Let's take some examples:

- Rename `node_cpu_seconds_total` to `vm_node_cpu_seconds_total` across all the
  scraped metrics:

  ```yaml
  metric_relabel_configs:
    - if: "node_cpu_seconds_total"
      replacement: vm_node_cpu_seconds_total
      target_label: __name__
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27node_cpu_seconds_total%27%0A++replacement%3A+vm_node_cpu_seconds_total%0A++target_label%3A+__name__&labels=%7B__name__%3D%22node_cpu_seconds_total%22%2C+cpu%3D%220%22%2C+mode%3D%22idle%22%7D)

- Rename all metrics starting with `http_` to start with `web_` instead (e.g.
  `http_requests_total` → `web_requests_total`):

  ```yaml
  metric_relabel_configs:
    - source_labels: [__name__]
      regex: "http_(.*)"
      replacement: web_$1
      target_label: __name__
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5B__name__%5D%0A++regex%3A+%27http_%28.*%29%27%0A++replacement%3A+web_%241%0A++target_label%3A+__name__&labels=%7B__name__%3D%22http_response_time_seconds%22%2C+method%3D%22GET%22%7D)

- Replace all dashes (`-`) in metric names with underscores (`_`) (e.g.
  `nginx-ingress-latency` → `nginx_ingress_latency`):

  ```yaml
  metric_relabel_configs:
    - source_labels: [__name__]
      action: replace_all
      regex: "-"
      replacement: "_"
      target_label: __name__
  ```

  [Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+source_labels%3A+%5B__name__%5D%0A++action%3A+replace_all%0A++regex%3A+%27-%27%0A++replacement%3A+%27_%27%0A++target_label%3A+__name__&labels=%7B__name__%3D%22nginx-ingress-latency%22%2C+host%3D%22example.com%22%7D)

{{% /collapse %}}

{{% collapse name="How to drop metrics during scrape" %}}

#### How to drop metrics during scrape

All examples above work at the label level: adding, dropping, or changing label
values of scraped metrics. You can also drop entire metrics. This is especially
beneficial for metrics that result in
[high cardinality](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-cardinality)
or
[high churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate).

Instead of `labeldrop` or `labelkeep` actions, we use `drop` or `keep` actions
in the `metric_relabel_configs` section:

- `action: drop`: drops all metrics that match the `if`
  [series selector](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering)
- `action: keep`: drops all metrics that don't match the `if`
  [series selector](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#filtering)

For example, the following config drops all metrics with names starting with
`container_`:

```yaml
metric_relabel_configs:
  - if: '{__name__=~"container_.*"}'
    action: drop
```

[Try the above config](https://play.victoriametrics.com/select/0/prometheus/graph/#/relabeling?config=-+if%3A+%27%7B__name__%3D%7E%22container_.*%22%7D%27%0A++action%3A+drop&labels=%7B__name__%3D%22container_memory_usage_bytes%22%2Ccontainer%3D%22nginx%22%2C+pod%3D%22web-1%22%7D)

Note that the relabeling config is specified under the `metric_relabel_configs`
section instead of `relabel_configs` section. They serve different purposes:

- The `scrape_configs[].relabel_configs` apply before scraping, modifying or
  filtering targets. Any changes here affect all metrics from that target.
- The `scrape_configs[].metric_relabel_configs` apply after scraping, modifying
  or filtering individual metrics.

{{% /collapse %}}

## Useful tips for target relabeling

- Target relabeling can be debugged by clicking the `debug` link for a target on
  the `http://vmagent:8429/target` or `http://vmagent:8429/service-discovery`
  pages. See
  [Relabel Debug - vmagent](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabel-debugging).
- Special labels with the `__` prefix are automatically added when discovering
  targets and removed after relabeling:
  - Meta-labels starting with the `__meta_` prefix. The specific sets of labels
    for each supported service discovery option are listed in
    [Prometheus Service Discovery](https://docs.victoriametrics.com/victoriametrics/sd_configs/#prometheus-service-discovery).
  - Additional labels with the `__` prefix other than `__meta_` labels, such as
    [`__scheme__` or `__address__`](#how-to-modify-scrape-urls-in-targets). It
    is common practice to store temporary labels with names starting with `__`
    during target relabeling.
- All target-level labels are automatically added to all metrics scraped from
  targets.
- The list of discovered scrape targets with all discovered meta-labels is
  available on the `http://vmagent:8429/service-discovery` page for `vmagent`
  and on the `http://victoriametrics:8428/service-discovery` page for
  single-node VictoriaMetrics.
- The list of active targets with the final set of target-labels after
  relabeling is available on the `http://vmagent:8429/targets` page for
  `vmagent` and on the `http://victoriametrics:8428/targets` page for
  single-node VictoriaMetrics.

## Useful tips for metric relabeling

- Metric relabeling can be debugged on the
  `http://vmagent:8429/metric-relabel-debug` page. See
  [these docs](https://docs.victoriametrics.com/victoriametrics/relabeling/#relabel-debugging).
- All labels that start with the `__` prefix are automatically removed from
  metrics after relabeling. It is common practice to store temporary labels with
  names starting with `__` during metrics relabeling.
- All target-level labels are automatically added to all metrics scraped from
  targets, making them available during metrics relabeling.
- If too many labels are removed, different metrics might look the same — this
  can lead to duplicate time series with conflicting values, which is usually a
  problem.
