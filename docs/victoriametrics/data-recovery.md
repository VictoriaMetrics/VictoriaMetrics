---
weight: 21
title: Recovering missing data
description: "Detect data gaps and copy missing samples from another VictoriaMetrics installation."
menu:
  docs:
    parent: victoriametrics
    weight: 21
tags:
  - metrics
  - guide
---

If one VictoriaMetrics installation is missing samples that another installation still has, `vmctl vm-native` can copy the affected time range from the healthy source. This applies to single-node and cluster installations, including independent clusters in different availability zones. The source must contain the missing samples; `vmctl` cannot recreate data that is absent from every installation.

## How gaps can occur

By default, `vmagent` keeps a separate [persistent queue](https://docs.victoriametrics.com/victoriametrics/vmagent/#on-disk-persistence) for each remote-write destination while it cannot deliver data promptly. A destination can miss samples when:

* Its queue is deleted or lost before the pending data is delivered, for example when the queue is on ephemeral storage.
* The queue reaches `-remoteWrite.maxDiskUsagePerURL` and `vmagent` discards its oldest buffered data.
* On-disk persistence is disabled and `vmagent` drops samples while the destination cannot keep up. A destination may also reject data with `400 Bad Request` or `409 Conflict` responses.

A growing queue or a temporary remote-write failure does not by itself mean data has been lost. Check whether the queue drains after delivery resumes. Fix the cause of data loss before copying historical samples, so the destination does not continue to develop gaps.

## Find the missing range

Inspect the affected `vmagent`'s logs and [monitoring metrics](https://docs.victoriametrics.com/victoriametrics/vmagent/#monitoring). `vmagent_remotewrite_pending_data_bytes` shows pending data for each destination; `vmagent_remotewrite_push_failures_total`, `vmagent_remotewrite_samples_dropped_total`, and `vmagent_remotewrite_packets_dropped_total` can help identify delivery failures or discarded data. These signals help locate an incident, but they do not establish the exact missing time range.

Query the same tenant and representative series on both installations over the suspected period. Use the same query and time resolution on each side, and check that the source has samples where the destination does not. For regularly scraped targets, compare `count_over_time(up{job="example"}[5m])` on both sides after replacing `job="example"` with a real target selector. A lower count at the destination indicates a possible gap; for pushed data, use a series that normally arrives at regular intervals instead. Inspect individual sample timestamps around the first and last difference to set a range that covers all missing samples. Do not derive the range only from the queue loss time or disk limit: data may have waited in the queue before it was lost. If delivery is still delayed, wait for the pending queue to drain before deciding which samples remain missing.

## Copy the data

The following example copies a time range for tenant `0:0` between two VictoriaMetrics clusters. Replace the addresses and UTC times with the endpoints and range confirmed above:

```sh
HEALTHY_READ_ADDR='http://healthy-vmselect.example.com:8481/select/0:0/prometheus'
AFFECTED_WRITE_ADDR='http://affected-vminsert.example.com:8480/insert/0:0/prometheus'
START='2025-01-01T00:00:00Z'
END='2025-01-02T00:00:00Z'

./vmctl vm-native \
  --vm-native-src-addr="${HEALTHY_READ_ADDR}" \
  --vm-native-dst-addr="${AFFECTED_WRITE_ADDR}" \
  --vm-native-filter-time-start="${START}" \
  --vm-native-filter-time-end="${END}"
```

For clusters, use `/select/<accountID>[:<projectID>]/prometheus` on the source and `/insert/<accountID>[:<projectID>]/prometheus` on the destination, with the same tenant ID in both paths. If `vmauth` fronts either endpoint, its route must forward the corresponding tenant-specific path. For single-node installations, use their base URLs. See [vmctl vm-native](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/) for authentication, TLS, and other filtering options. Copy one tenant at a time; the multitenant cluster paths are not suitable for this example.

`vmctl` copies samples in the selected range, including those already present at the destination. Overlap can create duplicates. On a cluster destination, set `-dedup.minScrapeInterval=1ms` or higher on `vmselect` and `vmstorage` if it is not already configured; queries then ignore samples with identical timestamps, although the duplicate samples remain in storage. See [replication and data safety](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety). For a single-node destination, configure the same flag on that instance if needed.

After `vmctl` reports `Import finished`, query the affected installation again with the same tenant, series, and time resolution used to find the gap. Confirm that the expected samples now appear throughout the copied range and that normal ingestion has resumed.
