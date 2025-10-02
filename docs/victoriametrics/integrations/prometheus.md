---
title: Prometheus
weight: 2
menu:
  docs:
    parent: "integrations-vm"
    identifier: "integrations-prometheus-vm"
    weight: 2
aliases:
  - /victoriametrics/data-ingestion/prometheus/
  - /data-ingestion/prometheus/
---

VictoriaMetrics integrates with Prometheus as [remote storage for writes](https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage).

To send data from Prometheus to VictoriaMetrics, add the following lines to your Prometheus config file:
```yaml
remote_write:
  - url: http://<victoriametrics-addr>:8428/api/v1/write
```
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For cluster version use vminsert address:
```
http://<vminsert-addr>:8480/insert/<tenant>/prometheus
```
_Replace `<vminsert-addr>` with the hostname or IP address of vminsert service._

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

Then apply the new config restart Prometheus or [hot-reload](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration) it:
```sh
kill -HUP `pidof prometheus`
```

Prometheus stores incoming data locally and also sends a copy to the remote storage.
This means even if the remote storage is down, data is still available locally for as long as set in `--storage.tsdb.retention.time`.

If you send data from more than one Prometheus instance, add the following lines into `global` section
of [Prometheus config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file):
```yaml
global:
  external_labels:
    datacenter: dc-123
```

This adds a label `datacenter=dc-123` to each sample sent to remote storage.
You can use any label name; datacenter is just an example. The value should be different for each Prometheus instance 
so you can filter and group time series by it.

For Prometheus servers handling a high load (200k+ samples per second), use these settings:
```yaml
remote_write:
  - url: http://<victoriametrics-addr>:8428/api/v1/write
    queue_config:
      max_samples_per_send: 10000
      capacity: 20000
      max_shards: 30
```

Using remote write can increase Prometheus memory use by around 25%. If memory usage is too high, try reducing 
`max_samples_per_send` and `capacity` params. These two settings work closely together, so adjust carefully.
Read more about tuning [remote write](https://prometheus.io/docs/practices/remote_write) for Prometheus.

It is recommended upgrading Prometheus to [v2.12.0](https://github.com/prometheus/prometheus/releases/latest) or newer,
since previous versions may have issues with `remote_write`.

Take a look at [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) and [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/),
which can be used as faster and less resource-hungry alternative to Prometheus.
