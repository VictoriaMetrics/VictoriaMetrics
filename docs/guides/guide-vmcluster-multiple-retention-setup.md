---
weight: 10
title: Multi Retention Setup within VictoriaMetrics Cluster
menu:
  docs:
    parent: "guides"
    weight: 10
aliases:
- /guides/guide-vmcluster-multiple-retention-setup.html
---
# Multi Retention Setup within VictoriaMetrics Cluster


**Objective**

Setup Victoria Metrics Cluster with support of multiple retention periods within one installation.

**Enterprise Solution**

[VictoriaMetrics enterprise](https://docs.victoriametrics.com/enterprise.html) supports specifying multiple retentions
for distinct sets of time series and [tenants](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#multitenancy)
via [retention filters](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#retention-filters).

**Open Source Solution**

Community version of VictoriaMetrics supports only one retention period per `vmstorage` node via [-retentionPeriod](https://docs.victoriametrics.com/#retention) command-line flag.

A multi-retention setup can be implemented by dividing a [victoriametrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) into logical groups with different retentions.

Example:
Setup should handle 3 different retention groups 3months, 1year and 3 years.
Solution contains 3 groups of vmstorages + vminserts and one group of vmselects. Routing is done by [vmagent](https://docs.victoriametrics.com/vmagent.html) and [relabeling configuration](https://docs.victoriametrics.com/vmagent.html#relabeling). The [-retentionPeriod](https://docs.victoriametrics.com/#retention) sets how long to keep the metrics.

The diagram below shows a proposed solution

<p align="center">
  <img src="guide-vmcluster-multiple-retention-setup.webp" width="800">
</p>

**Implementation Details**

1. Groups of vminserts A know about only vmstorages A and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup). 
1. Groups of vminserts B know about only vmstorages B and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup). 
1. Groups of vminserts C know about only vmstorages A and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup). 
1. Vmselect reads data from all vmstorage nodes via `-storageNode` [configuration](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup). 
1. Vmagent routes incoming metrics to the given set of `vminsert` nodes using relabeling rules specified at `-remoteWrite.urlRelabelConfig` [configuration](https://docs.victoriametrics.com/vmagent.html#relabeling).

**Multi-Tenant Setup**

Every group of vmstorages can handle one tenant or multiple one. Different groups can have overlapping tenants. As vmselect reads from all vmstorage nodes, the data is aggregated on its level.

**Additional Enhancements**

You can set up [vmauth](https://docs.victoriametrics.com/vmauth.html) for routing data to the given vminsert group depending on the needed retention.
