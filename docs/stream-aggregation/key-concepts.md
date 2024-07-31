---
sort: 2
weight: 2
title: Key concepts
menu:
  docs:
    identifier: stream-aggregation-key-concepts
    parent: 'stream-aggregation'
    weight: 2
aliases:
- /stream-aggregation/key-concepts/index.html
- /stream-aggretation/key-concepts/
---

[Single-node VictoriaMetrics](https://docs.victoriametrics.com/) supports relabeling,
deduplication and stream aggregation for all the received data, scraped or pushed. 
The processed data is then stored in local storage and **can't be forwarded further**.

[vmagent](https://docs.victoriametrics.com/vmagent) supports relabeling, deduplication and stream aggregation for all 
the received data, scraped or pushed. Then, the collected data will be forwarded to specified `-remoteWrite.url` destinations.
The data processing order is the following:
1. All the received data is [relabeled](https://docs.victoriametrics.com/vmagent#relabeling) according to 
   specified `-remoteWrite.relabelConfig`;
2. All the received data is [deduplicated](#deduplication)
   according to specified `-streamAggr.dedupInterval`;
3. All the received data is aggregated according to specified `-streamAggr.config`;
4. The resulting data from p1 and p2 is then replicated to each `-remoteWrite.url`;
5. Data sent to each `-remoteWrite.url` can be additionally relabeled according to the 
   corresponding `-remoteWrite.urlRelabelConfig` (set individually per URL);
6. Data sent to each `-remoteWrite.url` can be additionally deduplicated according to the
   corresponding `-remoteWrite.streamAggr.dedupInterval` (set individually per URL);
7. Data sent to each `-remoteWrite.url` can be additionally aggregated according to the
   corresponding `-remoteWrite.streamAggr.config` (set individually per URL). Please note, it is not recommended
   to use `-streamAggr.config` and `-remoteWrite.streamAggr.config` together, unless you understand the complications.

Typical scenarios for data routing with vmagent:
1. **Aggregate incoming data and replicate to N destinations**. For this one should configure `-streamAggr.config`
to aggregate the incoming data before replicating it to all the configured `-remoteWrite.url` destinations.
2. **Individually aggregate incoming data for each destination**. For this on should configure `-remoteWrite.streamAggr.config`
for each `-remoteWrite.url` destination. [Relabeling](https://docs.victoriametrics.com/vmagent#relabeling) 
via `-remoteWrite.urlRelabelConfig` can be used for routing only selected metrics to each `-remoteWrite.url` destination.

## Deduplication

[vmagent](https://docs.victoriametrics.com/vmagent) supports online [de-duplication](https://docs.victoriametrics.com#deduplication) of samples
before sending them to the configured `-remoteWrite.url`. The de-duplication can be enabled via the following options:

- By specifying the desired de-duplication interval via `-streamAggr.dedupInterval` command-line flag for all received data 
  or via `-remoteWrite.streamAggr.dedupInterval` command-line flag for the particular `-remoteWrite.url` destination.
  For example, `./vmagent -remoteWrite.url=http://remote-storage/api/v1/write -remoteWrite.streamAggr.dedupInterval=30s` instructs `vmagent` to leave
  only the last sample per each seen [time series](https://docs.victoriametrics.com/keyconcepts#time-series) per every 30 seconds.
  The de-deduplication is performed after applying [relabeling](https://docs.victoriametrics.com/vmagent#relabeling) and
  before performing the aggregation.
  If the `-remoteWrite.streamAggr.config` and / or `-streamAggr.config` is set, then the de-duplication is performed individually per each
  [stream aggregation config](./configuration/#configuration-file-reference) for the matching samples after applying [input_relabel_configs](#relabeling).

- By specifying `dedup_interval` option individually per each [stream aggregation config](./configuration/#configuration-file-reference) 
  in `-remoteWrite.streamAggr.config` or `-streamAggr.config` configs.

[Single-node VictoriaMetrics](https://docs.victoriametrics.com/) supports two types of de-duplication:
- After storing the duplicate samples to local storage. See [`-dedup.minScrapeInterval`](https://docs.victoriametrics.com/#deduplication) command-line option.
- Before storing the duplicate samples to local storage. This type of de-duplication can be enabled via the following options:
  - By specifying the desired de-duplication interval via `-streamAggr.dedupInterval` command-line flag.
    For example, `./victoria-metrics -streamAggr.dedupInterval=30s` instructs VictoriaMetrics to leave only the last sample per each
    seen [time series](https://docs.victoriametrics.com/keyconcepts#time-series) per every 30 seconds.
    The de-duplication is performed after applying [relabeling](https://docs.victoriametrics.com/#relabeling) and before performing the aggregation.

    If the `-remtoeWrite.streamAggr.config` and / or `-streamAggr.config` is set, then the de-duplication is performed individually per each
    [stream aggregation config](./configuration/#configuration-file-reference) for the matching samples after applying [input_relabel_configs](#relabeling).

  - By specifying `dedup_interval` option individually per each [stream aggregation config](./configuration/#configuration-file-reference)
    in `-remoteWrite.streamAggr.config` or `-streamAggr.config` configs.

It is possible to drop the given labels before applying the de-duplication. See [these docs](#dropping-unneeded-labels).

The online de-duplication uses the same logic as [`-dedup.minScrapeInterval` command-line flag](https://docs.victoriametrics.com#deduplication) at VictoriaMetrics.

## Ignoring old samples

By default, all the input samples are taken into account during stream aggregation. If samples with old timestamps 
outside the current [aggregation interval](./configuration/#interval) must be ignored, then the following options can be used:

- To pass `-streamAggr.ignoreOldSamples` command-line flag to [single-node VictoriaMetrics](https://docs.victoriametrics.com)
  or to [vmagent](https://docs.victoriametrics.com/vmagent). At [vmagent](https://docs.victoriametrics.com/vmagent)
  `-remoteWrite.streamAggr.ignoreOldSamples` flag can be specified individually per each `-remoteWrite.url`.
  This enables ignoring old samples for all the [aggregation configs](./configuration/#configuration-file-reference).

- To set [`ignore_old_samples:`](./configuration/#ignore-old-samples) `true` option at the particular [aggregation config](./configuration/#configuration-file-reference).
  This enables ignoring old samples for that particular aggregation config.

## Ignore aggregation intervals on start

Streaming aggregation results may be incorrect for some time after the restart of [vmagent](https://docs.victoriametrics.com/vmagent)
or [single-node VictoriaMetrics](https://docs.victoriametrics.com) until all the buffered [samples](https://docs.victoriametrics.com/keyconcepts#raw-samples)
are sent from remote sources to the `vmagent` or single-node VictoriaMetrics via [supported data ingestion protocols](https://docs.victoriametrics.com/vmagent#how-to-push-data-to-vmagent).
In this case it may be a good idea to drop the aggregated data during the first `N` [aggregation intervals](./configuration/#interval)
just after the restart of `vmagent` or single-node VictoriaMetrics. This can be done via the following options:

- Set `-streamAggr.ignoreFirstIntervals=<intervalsCount>` command-line flag to [single-node VictoriaMetrics](https://docs.victoriametrics.com)
  or to [vmagent](https://docs.victoriametrics.com/vmagent) to skip first `<intervalsCount>` [aggregation intervals](./configuration/#interval)
  from persisting to the storage. At [vmagent](https://docs.victoriametrics.com/vmagent)
  `-remoteWrite.streamAggr.ignoreFirstIntervals=<intervalsCount>` flag can be specified individually per each `-remoteWrite.url`.
  It is expected that all incomplete or queued data will be processed during specified `<intervalsCount>` 
  and all subsequent aggregation intervals will produce correct data.

- Set `ignore_first_intervals: <intervalsCount>` option individually per [aggregation config](./configuration/#configuration-file-reference).
  This enables ignoring first `<intervalsCount>` aggregation intervals for that particular aggregation config.

## Flush time alignment

By default, the time for aggregated data flush is aligned by the [`interval`](./configuration/#interval) option.

For example:

- if `interval: 1m` is set, then the aggregated data is flushed to the storage at the end of every minute
- if `interval: 1h` is set, then the aggregated data is flushed to the storage at the end of every hour

If you do not need such an alignment, then set [`no_align_flush_to_interval:`](./configuration/#no-align-flush-to-interval) `true` option in the [aggregate config](./configuration/#configuration-file-reference).
In this case aggregated data flushes will be aligned to the `vmagent` start time or to [config reload](./configuration/#configuration-update) time.

The aggregated data on the first and the last interval is dropped during `vmagent` start, restart or [config reload](./configuration/#configuration-update),
since the first and the last aggregation intervals are incomplete, so they usually contain incomplete confusing data.
If you need preserving the aggregated data on these intervals, then set [`flush_on_shutdown:`](./configuration/#flush-on-shutdown) `true` option.

See also:

- [Ignore aggregation intervals on start](#ignore-aggregation-intervals-on-start)
- [Ignoring old samples](#ignoring-old-samples)

## Output metric names

Output metric names for stream aggregation are constructed according to the following pattern:

```text
<metric_name>:<interval>[_by_<by_labels>][_without_<without_labels>]_<output>
```

- `<metric_name>` is the original metric name.
- `<interval>` is the [`interval`](./configuration/#interval) specified in the [stream aggregation config](./configuration/#configuration-file-reference).
- `<by_labels>` is `_`-delimited sorted list of [`by`](./configuration/#by) labels.
  If the [`by`](./configuration/#by) list is missing in the config, then the `_by_<by_labels>` part isn't included in the output metric name.
- `<without_labels>` is an optional `_`-delimited sorted list of [`without`](./configuration/#without) labels specified in the [stream aggregation config](./configuration/#configuration-file-reference).
  If the [`without`](./configuration/#without) list is missing in the config, then the `_without_<without_labels>` part isn't included in the output metric name.
- `<output>` is the aggregate used for constructing the output metric. The aggregate name is taken from the [`outputs`](./configuration/outputs) list
  at the corresponding [stream aggregation config](./configuration/#configuration-file-reference).

Both input and output metric names can be modified if needed via relabeling according to [these docs](#relabeling).

It is possible to leave the original metric name after the aggregation by specifying [`keep_metric_names:`](./configuration/#keep-metric-names) `true` option at [stream aggregation config](./configuration/#configuration-file-reference).
The [`keep_metric_names`](./configuration/#keep-metric-names) option can be used if only a single output is set in [`outputs`](./configuration/outputs) list.

## Relabeling

It is possible to apply [arbitrary relabeling](https://docs.victoriametrics.com/vmagent#relabeling) to input and output metrics
during stream aggregation via [`input_relabel_configs`](./configuration/#input-relabel-configs) and [`output_relabel_configs`](./configuration/#output-relabel-configs) options in [stream aggregation config](./configuration/#configuration-file-reference).

Relabeling rules inside [`input_relabel_configs`](./configuration/#input-relabel-configs) are applied to samples matching the [`match`](./configuration/#match) filters before optional [deduplication](#deduplication).
Relabeling rules inside [`output_relabel_configs`](./configuration/#output-relabel-configs) are applied to aggregated samples before sending them to the remote storage.

For example, the following config removes the `:1m_sum_samples` suffix added [to the output metric name](#output-metric-names):

```yaml
- interval: 1m
  outputs: [sum_samples]
  output_relabel_configs:
  - source_labels: [__name__]
    target_label: __name__
    regex: "(.+):.+"
```

Another option to remove the suffix, which is added by stream aggregation, is to add [`keep_metric_names:`](./configuration/#keep-metric-names) `true` to the config:

```yaml
- interval: 1m
  outputs: [sum_samples]
  keep_metric_names: true
```

See also [dropping unneeded labels](#dropping-unneeded-labels).


## Dropping unneeded labels

If you need dropping some labels from input samples before [input relabeling](#relabeling), [de-duplication](#deduplication)
and stream aggregation, then the following options exist:

- To specify comma-separated list of label names to drop in `-streamAggr.dropInputLabels` command-line flag
  or via `-remoteWrite.streamAggr.dropInputLabels` individually per each `-remoteWrite.url`.
  For example, `-streamAggr.dropInputLabels=replica,az` instructs to drop `replica` and `az` labels from input samples
  before applying de-duplication and stream aggregation.

- To specify [`drop_input_labels`](./configuration/#drop-input-labels) list with the labels to drop.
  For example, the following config drops `replica` label from input samples with the name `process_resident_memory_bytes`
  before calculating the average over one minute:

  ```yaml
  - match: process_resident_memory_bytes
    interval: 1m
    drop_input_labels: [replica]
    outputs: [avg]
    keep_metric_names: true
  ```

Typical use case is to drop `replica` label from samples, which are received from high availability replicas.

## Aggregating by labels

All the labels for the input metrics are preserved by default in the output metrics. For example,
the input metric `foo{app="bar",instance="host1"}` results to the output metric `foo:1m_sum_samples{app="bar",instance="host1"}`
when the following [stream aggregation config](./configuration/#configuration-file-reference) is used:

```yaml
- interval: 1m
  outputs: [sum_samples]
```

The input labels can be removed via [`without`](./configuration/#without) list specified in the config. For example, the following config
removes the `instance` label from output metrics by summing input samples across all the instances:

```yaml
- interval: 1m
  without: [instance]
  outputs: [sum_samples]
```

In this case the `foo{app="bar",instance="..."}` input metrics are transformed into `foo:1m_without_instance_sum_samples{app="bar"}`
output metric according to [output metric naming](#output-metric-names).

It is possible specifying the exact list of labels in the output metrics via [`by`](./configuration/#by) list.
For example, the following config sums input samples by the `app` label:

```yaml
- interval: 1m
  by: [app]
  outputs: [sum_samples]
```

In this case the `foo{app="bar",instance="..."}` input metrics are transformed into `foo:1m_by_app_sum_samples{app="bar"}`
output metric according to [output metric naming](#output-metric-names).

The labels used in [`by`](./configuration/#by) and [`without`](./configuration/#without) lists can be modified via [`input_relabel_configs`](./configuration/#input-relabel-configs) section - see [these docs](#relabeling).

See also [aggregation outputs](./configuration/outputs/).
