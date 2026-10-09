---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
If one VictoriaMetrics installation is missing samples that another installation still has, `vmctl vm-native` can copy the affected time range from the healthy source. This applies to single-node and cluster installations, including independent clusters in different availability zones. The source must contain the missing samples; `vmctl` cannot recreate data that is absent from every installation.

In a typical [Multi-Cluster and Multi-AZ topology](https://docs.victoriametrics.com/guides/vm-architectures/#multi-cluster-and-multi-az), independent VictoriaMetrics clusters run in separate availability zones. `vmagent` collects or receives samples and replicates them to each cluster through separate remote-write destinations. Each destination has its own queue, so a slow or unavailable cluster can fall behind while the other clusters continue receiving data.

## How gaps can occur

By default, `vmagent` keeps a separate [persistent queue](https://docs.victoriametrics.com/victoriametrics/vmagent/#on-disk-persistence) for each remote-write destination while it cannot deliver data promptly. A destination can miss samples when:

* Its queue is deleted or lost before the pending data is delivered, for example when the queue is on ephemeral storage.
* `-remoteWrite.url` values or their order change while queues hold pending data, and `-remoteWrite.keepDanglingQueues` isn't set; `vmagent` deletes the orphaned queues on restart.
* The queue reaches `-remoteWrite.maxDiskUsagePerURL` and `vmagent` discards its oldest buffered data.
* On-disk persistence is disabled and a destination cannot keep up. Depending on the input type and flags, `vmagent` may reject pushed requests with `429 Too Many Requests` or drop samples instead. Data that exists only in memory can also be lost if `vmagent` stops ungracefully. See [disabling on-disk persistence](https://docs.victoriametrics.com/victoriametrics/vmagent/#disabling-on-disk-persistence).
* Remote storage rejects a block with `400 Bad Request` or `409 Conflict`; `vmagent` drops blocks with these responses instead of retrying them.

Fix the cause of data loss before copying historical samples, so the destination does not continue to develop gaps.

## Find the missing range

First, check whether data was dropped. Inspect the affected `vmagent`'s logs and [monitoring metrics](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring) for the affected remote-write destination:

* Increases in `vm_persistentqueue_bytes_dropped_total` indicate data discarded from the persistent queue when its disk limit is reached. Match its `path` label to the queue's `path` in `vmagent_remotewrite_pending_data_bytes`.
* Increases in `vmagent_remotewrite_samples_dropped_total` indicate samples dropped when the queue cannot accept them, and `vmagent_remotewrite_packets_dropped_total` indicates blocks rejected by remote storage.
* `vm_app_prev_shutdown_unclean` equal to `1` indicates that the previous `vmagent` run stopped ungracefully, so its in-memory buffers may have been lost.
* Check for a lost or deleted queue separately: its contents can be lost without an increase in these counters.

A growing queue or an increase in `vmagent_remotewrite_push_failures_total` does not by itself mean data has been lost. Storage can remain available for queries while it is too saturated to ingest data promptly. If no data was dropped and the queue is intact, wait for `vmagent_remotewrite_pending_data_bytes` to drain after delivery resumes; copying historical data is not needed.

If data was dropped, use the history of `vmagent_remotewrite_pending_data_bytes` to find when the affected queue started accumulating and when it started draining without growing again. Select a time range covering that period, then widen it slightly on both sides to include samples buffered before the observed increase and around the recovery. Do not use only the time of a drop or queue deletion: discarded samples may have waited in the queue before that event. Account for any existing backlog when choosing the start time.

Confirm that the source contains data throughout the selected range and that it is within the destination's retention period. Use the same tenant and representative series when querying both installations. A query such as `count_over_time(up{job="example"}[5m])` can help compare a regularly scraped target after replacing the selector with a real one, but it cannot establish the extent of data loss across all series and tenants. Use the queue history to choose the recovery range rather than narrowing it to differences in one series.

## Copy metrics - Cluster installations

The following example copies all tenants' data for the selected range between two VictoriaMetrics clusters. Replace the addresses and UTC times with the endpoints and range identified above:

```sh
HEALTHY_READ_ADDR='http://healthy-vmselect.example.com:8481'
AFFECTED_WRITE_ADDR='http://affected-vminsert.example.com:8480'
START='2025-01-01T00:00:00Z'
END='2025-01-02T00:00:00Z'

./vmctl vm-native \
  --vm-native-src-addr="${HEALTHY_READ_ADDR}" \
  --vm-native-dst-addr="${AFFECTED_WRITE_ADDR}" \
  --vm-native-filter-time-start="${START}" \
  --vm-native-filter-time-end="${END}" \
  --vm-intercluster
```

`--vm-intercluster` discovers tenants through the source's `/admin/tenants` endpoint and copies each tenant to the same tenant ID at the destination. Use the base URLs of `vmselect` and `vminsert`, without tenant paths. If `vmauth` fronts either endpoint, its routes must allow tenant discovery on the source and the corresponding tenant-specific export and import paths. See [cluster-to-cluster migration](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/#cluster-to-cluster) for details.

## Copy metrics - Single-node installations

For two single-node VictoriaMetrics installations, use their base URLs and omit `--vm-intercluster`:

```sh
HEALTHY_READ_ADDR='http://healthy-victoriametrics.example.com:8428'
AFFECTED_WRITE_ADDR='http://affected-victoriametrics.example.com:8428'
START='2025-01-01T00:00:00Z'
END='2025-01-02T00:00:00Z'

./vmctl vm-native \
  --vm-native-src-addr="${HEALTHY_READ_ADDR}" \
  --vm-native-dst-addr="${AFFECTED_WRITE_ADDR}" \
  --vm-native-filter-time-start="${START}" \
  --vm-native-filter-time-end="${END}"
```

See [vmctl vm-native](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/) for authentication, TLS, and other filtering options.

`vmctl` copies samples in the selected range, including those already present at the destination. Overlap can create duplicates. On a cluster destination, set `-dedup.minScrapeInterval=1ms` on `vmselect` and `vmstorage` if it is not already configured; queries then ignore samples with identical timestamps, although the duplicate samples remain in storage. See [replication and data safety](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety). For a single-node destination, configure the same flag on that instance if needed.

After `vmctl` reports `Import finished`, query the affected installation again. Check the expected series in each affected tenant throughout the copied range and confirm that normal ingestion has resumed.

## Recap

1. Fix the cause of data loss so the destination stops developing new gaps.
1. Confirm that data was actually dropped; a growing queue alone doesn't mean data was lost.
1. Use the queue history to choose the time range to recover, and widen it slightly on both sides.
1. Check that the healthy source holds data for the whole range.
1. Copy the range with `vmctl vm-native`, adding `--vm-intercluster` for clusters.
1. Enable deduplication at the destination and verify the copied data.
