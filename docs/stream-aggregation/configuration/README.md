Stream aggregation can be configured via the following command-line flags:

- `-streamAggr.config` at [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
  and at [vmagent](https://docs.victoriametrics.com/vmagent).
- `-remoteWrite.streamAggr.config` at [vmagent](https://docs.victoriametrics.com/vmagent) only.
  This flag can be specified individually per each `-remoteWrite.url` and aggregation will happen independently for each of them.
  This allows writing different aggregates to different remote storage destinations.

These flags must point to a file containing [stream aggregation config](#configuration-file-reference).
The file may contain `%{ENV_VAR}` placeholders which are substituted by the corresponding `ENV_VAR` environment variable values.

By default, the following data is written to the storage when stream aggregation is enabled:

- the aggregated samples;
- the raw input samples, which didn't match any [`match`](#match) option in the provided [config](#configuration-file-reference)

This behaviour can be changed via the following command-line flags:

- `-streamAggr.keepInput` at [single-node VictoriaMetrics](https://docs.victoriametrics.com)
  and [vmagent](https://docs.victoriametrics.com/vmagent). At [vmagent](https://docs.victoriametrics.com/vmagent)
  `-remoteWrite.streamAggr.keepInput` flag can be specified individually per each `-remoteWrite.url`.
  If one of these flags is set, then all the input samples are written to the storage alongside the aggregated samples.
- `-streamAggr.dropInput` at [single-node VictoriaMetrics](https://docs.victoriametrics.com)
  and [vmagent](https://docs.victoriametrics.com/vmagent). At [vmagent](https://docs.victoriametrics.com/vmagent)
  `-remoteWrite.streamAggr.dropInput` flag can be specified individually per each `-remoteWrite.url`.
  If one of these flags are set, then all the input samples are dropped, while only the aggregated samples are written to the storage.

## Configuration File Reference

### Overview

Stream aggregation config file contains YAML formatted configs and may be referred via
`-streamAggr.config` command-line flag at [single-node VictoriaMetrics](https://docs.victoriametrics.com)
and [vmagent](https://docs.victoriametrics.com/vmagent). At [vmagent](https://docs.victoriametrics.com/vmagent) `-remoteWrite.streamAggr.config`
command-line flag can be specified individually per each `-remoteWrite.url` or just once, which
enables same aggregation rules for each `-remoteWrite.url`.

### Example configuration

```yaml
- match: 'http_request_duration_seconds_bucket{env=~"prod|staging"}'
  interval: 1m
  by: [vmrange]
  outputs: [total]
```
* [`name`](?selfref=true#name) `(string: "none")` - name of the given streaming aggregation config. 
  If it is set, then it is used as `name` label in the exposed metrics for the given aggregation config at /metrics page.
  See monitoring related information [here](https://docs.victoriametrics.com/vmagent#monitoring) and [here](https://docs.victoriametrics.com/#monitoring)
* [`match`](?selfref=true#match) `(list<string> or string: [])` - an optional filter for incoming samples to aggregate.
  It can contain arbitrary Prometheus series selector
  according to [filtering concepts](https://docs.victoriametrics.com/keyconcepts#filtering).
  If match isn't set, then all the incoming samples are aggregated.
  match also can contain a list of series selectors. Then the incoming samples are aggregated
  if they match at least a single series selector.
* [`interval`](?selfref=true#interval)  `(string: "", required)` - interval for the aggregation. The aggregated stats are sent to 
  remote storage once per interval.
* [`dedup_interval`](?selfref=true#dedup_interval) `(string:"")` - interval for de-duplication of input samples before the aggregation.
  Samples are de-duplicated on a per-series basis. See [timeseries](https://docs.victoriametrics.com/keyconcepts#time-series) and [deduplication](#deduplication)
  The deduplication is performed after [`input_relabel_configs`](#input-relabel-configs) relabeling is applied. By default, the deduplication is disabled
  unless `-remoteWrite.streamAggr.dedupInterval` or `-streamAggr.dedupInterval` command-line flags are set.
* [`staleness_interval`](?selfref=true#staleness_interval) `(string:2*interval)` - interval for resetting the per-series state if no new samples
  are received during this interval for the following outputs:
  * [`rate_avg`](./outputs/#rate_avg)
  * [`rate_sum`](./outputs/#rate_sum)
  * [`total`](./outputs/#total)
  * [`total_prometheus`](./outputs/#total_prometheus)
  * [`increase`](./outputs/#increase)
  * [`increase_prometheus`](./outputs/#increase_prometheus)
  * [`histogram_bucket`](./outputs/#histogram_bucket)
  Check [staleness](#staleness) for more details.
* [`no_align_flush_to_interval`](?selfref=true#no_align_flush_to_interval) `(bool: false)` - disables aligning of flush times for the aggregated data to multiples of interval.
  By default, flush times for the aggregated data is aligned to multiples of interval.
  For example:
  * if [`interval:`](#interval) `1m` is set, then flushes happen at the end of every minute,
  * if [`interval:`](#interval) `1h` is set, then flushes happen at the end of every hour
* [`flush_on_shutdown`](?selfref=true#flush_on_shutdown) `(bool: false)` - instructs to flush aggregated data to the storage on the first and the last intervals
  during vmagent starts, restarts or configuration reloads.
  Incomplete aggregated data isn't flushed to the storage by default, since it is usually confusing.
* [`without`](?selfref=true#without) `(list<string>: [])` - list of labels, which must be removed from the output aggregation.
  See [aggregation by labels](#aggregating-by-labels)
* [`by`](?selfref=true#by) `(list<string>: [])` - list of labels, which must be preserved in the output aggregation.
  See [aggregation by labels](#aggregating-by-labels)
* [`outputs`](?selfref=true#outputs) `(list<string>:[], required)` - list of aggregations to perform on the input data.
  See [aggregation outputs](#outputs).
* [`keep_metric_names`](?selfref=true#keep_input_names) `(bool: false)` - instructs keeping the original metric names for the aggregated samples.
  This option can be set only if outputs list contains only a single output.
  By default, a special suffix is added to original metric names in the aggregated samples.
  See [output metric names](../#output-metric-names)
* [`ignore_old_samples`](?selfref=true#ignore_old_samples) `(bool: false)` - instructs ignoring input samples with old timestamps outside the current aggregation interval.
  See [ignoring old samples](../#ignoring-old-samples)
  See also [-remoteWrite.streamAggr.ignoreOldSamples](#remote-write-ignore-old-samples-flag) or [-streamAggr.ignoreOldSamples](#ignore-old-samples-flag) command-line flag.
* [`ignore_first_intervals`](?selfref=true#ignore_first_intervals) `(int: 0)` - instructs ignoring the first N aggregation intervals after process start.
  See [ignore first intervals on start](../#ignore-aggregation-intervals-on-start)
  See also [-remoteWrite.streamAggr.ignoreFirstIntervals](#remote-write-ignore-first-intervals-flag) or [-streamAggr.ignoreFirstIntervals](#ignore-first-intervals-flag) command-line flags.
* [`drop_input_labels`](?selfref=true#drop_input_labels) `(bool: false)` - instructs dropping the given labels from input samples.
  The labels' dropping is performed before [`input_relabel_configs`](#input-relabel-configs) are applied.
  This also means that the labels are dropped before [deduplication](../#deduplication) and stream aggregation.
* [`input_relabel_configs`](?selfref=true#output_relabel_configs) `(array<relabel_config>: [])` - relabeling rules, which are applied to the incoming samples
  after they pass the match filter and before being aggregated. See [relabeling](../#relabeling)
* [`output_relabel_configs`](?selfref=true#output_relabel_configs) `(array<relabel_config>: [])` - relabeling rules, which are applied to the aggregated output metrics.

The file can contain multiple aggregation configs. The aggregation is performed independently
per each specified config entry.

### Configuration update

[vmagent](https://docs.victoriametrics.com/vmagent) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/vmagent)
support the following approaches for hot reloading stream aggregation configs from `-remoteWrite.streamAggr.config` and `-streamAggr.config`:

* By sending `SIGHUP` signal to `vmagent` or `victoria-metrics` process:

  ```sh
  kill -SIGHUP `pidof vmagent`
  ```

* By sending HTTP request to `/-/reload` endpoint (e.g. `http://vmagent:8429/-/reload` or `http://victoria-metrics:8428/-/reload).
