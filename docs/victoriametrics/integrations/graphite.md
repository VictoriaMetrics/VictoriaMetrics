---
title: Graphite
weight: 3
menu:
  docs:
    parent: "integrations-vm"
    weight: 3
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive inserts from Graphite-compatible agents (such as [StatsD](https://github.com/etsy/statsd)).
VictoriaMetrics **single-node** and **vmselect** support Graphite query language. It can be integrated with Grafana via [Graphite datasource](https://grafana.com/docs/grafana/latest/datasources/graphite/).
See [How Grammarly Improved Monitoring by Over 10x with VictoriaMetrics](https://www.grammarly.com/blog/engineering/monitoring-with-victoriametrics/)
by switching from Graphite and keeping all the same Grafana dashboards and agents.

See full list of Graphite-related configuration flags by running:
```sh
/path/to/victoria-metrics-prod --help | grep graphite
```

## Ingesting

Enable Graphite receiver by setting `-graphiteListenAddr` command line flag:
```sh
/path/to/victoria-metrics-prod -graphiteListenAddr=:2003
```

Now, VictoriaMetrics host name and specified port can be used as destination address in Graphite-compatible agents
(e.g.  `graphiteHost`  in `StatsD`).

Try writing a data sample via Graphite plaintext protocol to local VictoriaMetrics using `nc`:
```sh
echo "foo.bar.baz;tag1=value1;tag2=value2 123 `date +%s`" | nc -N localhost 2003
```

Metrics can be sanitized during ingestion according to Prometheus naming convention by passing `-graphite.sanitizeMetricName` command-line flag
to VictoriaMetrics. The following modifications are applied to the ingested samples when this flag is passed to VictoriaMetrics:
* remove redundant dots, e.g: `metric..name` => `metric.name`
* replace characters not matching `a-zA-Z0-9:_.` chars with `_`

VictoriaMetrics sets the current time to the ingested samples if the timestamp is omitted.
An arbitrary number of lines delimited by `\n` (aka newline char) can be sent in one go.

VictoriaMetrics single-node or vmselect can read the ingested data back.
Try reading the data via [/api/v1/export](https://docs.victoriametrics.com/victoriametrics/#how-to-export-data-in-json-line-format) endpoint:
```sh
curl -G 'http://localhost:8428/api/v1/export' -d 'match=foo.bar.baz'
```
_Note, we're using :8428 port here, as it is default port where VictoriaMetrics single-node listens for user requests._

The `/api/v1/export` endpoint should return the following response:
```json
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277406000]}
```

See also [Graphite relabeling](https://docs.victoriametrics.com/victoriametrics/relabeling/#graphite-relabeling).

## Querying

VictoriaMetrics **single-node** or **vmselect** support the following query APIs:
* [Graphite API](#graphite-api-usage)
* [Prometheus querying API](https://docs.victoriametrics.com#prometheus-querying-api-usage). See also [selecting Graphite metrics](#selecting-graphite-metrics).
* [go-graphite/carbonapi](https://github.com/go-graphite/carbonapi/blob/main/cmd/carbonapi/carbonapi.example.victoriametrics.yaml)

## Selecting Graphite metrics

VictoriaMetrics supports `__graphite__` pseudo-label for selecting time series with Graphite-compatible filters in [MetricsQL](https://docs.victoriametrics.com/victoriametrics/metricsql/).
For example, `{__graphite__="foo.*.bar"}` is equivalent to `{__name__=~"foo[.][^.]*[.]bar"}`, but works faster and is easier 
to use when migrating from Graphite to VictoriaMetrics. See [docs for Graphite paths and wildcards](https://graphite.readthedocs.io/en/latest/render_api.html#paths-and-wildcards).
VictoriaMetrics also supports [label_graphite_group](https://docs.victoriametrics.com/victoriametrics/metricsql/#label_graphite_group) 
function for extracting the given groups from Graphite metric name.

The `__graphite__` pseudo-label supports e.g. alternate regexp filters such as `(value1|...|valueN)`.
They are transparently converted to `{value1,...,valueN}` syntax [used in Graphite](https://graphite.readthedocs.io/en/latest/render_api.html#paths-and-wildcards). 
This allows using [multi-value template variables in Grafana](https://grafana.com/docs/grafana/latest/variables/formatting-multi-value-variables/) 
inside `__graphite__` pseudo-label. For example, Grafana expands `{__graphite__=~"foo.($bar).baz"}` into `{__graphite__=~"foo.(x|y).baz"}` 
if `$bar` template variable contains `x` and `y` values. In this case the query is automatically converted 
into `{__graphite__=~"foo.{x,y}.baz"}` before execution.

VictoriaMetrics also supports Graphite query language - see [Render API](#render-api).

## Graphite API usage

To integrate with [Graphite datasource in Grafana](https://grafana.com/docs/grafana/latest/datasources/graphite/),
VictoriaMetrics supports the following Graphite querying APIs:
* [Render API](#render-api)
* [Metrics API](#metrics-api).
* [Tags API](#tags-api).

All Graphite handlers can be prepended with `/graphite` prefix. For example, both `/graphite/metrics/find` and `/metrics/find` should work.

VictoriaMetrics accepts optional query args: `extra_label=<label_name>=<label_value>` and `extra_filters[]=series_selector`
for all the Graphite APIs. These args can be used for limiting the scope of time series visible to the given tenant.
It is expected that the `extra_label` query arg is automatically set by auth proxy sitting in front of VictoriaMetrics.
See [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/) and [vmgateway](https://docs.victoriametrics.com/victoriametrics/vmgateway/) as examples of such proxies.

[Contact us](mailto:sales@victoriametrics.com) if you need assistance with such a proxy.

VictoriaMetrics supports `__graphite__` pseudo-label for filtering time series with Graphite-compatible filters 
in [MetricsQL](https://docs.victoriametrics.com/victoriametrics/metricsql/). See [Selecting Graphite metrics](#selecting-graphite-metrics).

### Render API

VictoriaMetrics supports [Graphite Render API](https://graphite.readthedocs.io/en/stable/render_api.html) subset
at `/render` endpoint, which is used by [Graphite datasource in Grafana](https://grafana.com/docs/grafana/latest/datasources/graphite/).
When configuring Graphite datasource in Grafana, the `Storage-Step` HTTP request header must be set to a step between Graphite data points
stored in VictoriaMetrics. For example, `Storage-Step: 10s` would mean 10 seconds distance between Graphite datapoints stored in VictoriaMetrics.

#### Known Incompatibilities with `graphite-web`

- **Timestamp Shifting**: VictoriaMetrics does not support shifting response timestamps outside the request time range 
  as `graphite-web` does. This limitation impacts chained functions with time modifiers, such as `timeShift(summarize)`. 
  For more details, refer to this [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2969).

- **Non-deterministic series order**: due to the distributed nature of metrics processing, functions within the `seriesLists`
  family can produce non-deterministic results. To ensure consistent results, arguments for these functions must be 
  wrapped with a sorting function. For instance, the function `divideSeriesLists(series_list_1, series_list_2)` 
  should be modified to `divideSeriesLists(sortByName(series_list_1), sortByName(series_list_2))`.
  See this [issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5810) for details.
  The affected functions include:
  - `aggregateSeriesLists`
  - `diffSeriesLists`
  - `multiplySeriesLists`
  - `divideSeriesLists`

### Metrics API

VictoriaMetrics supports the following handlers from [Graphite Metrics API](https://graphite-api.readthedocs.io/en/latest/api.html#the-metrics-api):
* [/metrics/find](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find)
* [/metrics/expand](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-expand)
* [/metrics/index.json](https://graphite-api.readthedocs.io/en/latest/api.html#metrics-index-json)

VictoriaMetrics accepts the following additional query args at `/metrics/find` and `/metrics/expand`:
* `label` - for selecting arbitrary label values. By default, `label=__name__`, i.e. metric names are selected.
* `delimiter` - for using different delimiters in metric name hierarchy. For example, `/metrics/find?delimiter=_&query=node_*`
  would return all the metric name prefixes that start with `node_`. By default `delimiter=.`.

### Tags API

VictoriaMetrics supports the following handlers from [Graphite Tags API](https://graphite.readthedocs.io/en/stable/tags.html):

* [/tags/tagSeries](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb)
* [/tags/tagMultiSeries](https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb)
* [/tags](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/{tag_name}](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/findSeries](https://graphite.readthedocs.io/en/stable/tags.html#exploring-tags)
* [/tags/autoComplete/tags](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support)
* [/tags/autoComplete/values](https://graphite.readthedocs.io/en/stable/tags.html#auto-complete-support)
* [/tags/delSeries](https://graphite.readthedocs.io/en/stable/tags.html#removing-series-from-the-tagdb)
