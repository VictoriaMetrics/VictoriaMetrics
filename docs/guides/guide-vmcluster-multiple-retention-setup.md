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
**Objective**

Setup Victoria Metrics Cluster with support of multiple retention periods within one installation.

**Enterprise Solution**

[VictoriaMetrics enterprise](../enterprise.md) supports specifying multiple retentions
for distinct sets of time series and [tenants](../Cluster-VictoriaMetrics.md#multitenancy)
via [retention filters](../Cluster-VictoriaMetrics.md#retention-filters).

**Open Source Solution**

Community version of VictoriaMetrics supports only one retention period per `vmstorage` node via [-retentionPeriod](../#retention) command-line flag.

A multi-retention setup can be implemented by dividing a [victoriametrics cluster](../Cluster-VictoriaMetrics.md) into logical groups with different retentions.

Example:
Setup should handle 3 different retention groups 3months, 1year and 3 years.
Solution contains 3 groups of vmstorages + vminserts and one group of vmselects. Routing is done by [vmagent](../vmagent.md)
by [splitting data streams](../vmagent.md#splitting-data-streams-among-multiple-systems).
The [-retentionPeriod](../#retention) sets how long to keep the metrics.

The diagram below shows a proposed solution

![Setup](guide-vmcluster-multiple-retention-setup.webp)

**Implementation Details**

1. Groups of vminserts A know about only vmstorages A and this is explicitly specified via `-storageNode` [configuration](../Cluster-VictoriaMetrics.md#cluster-setup).
1. Groups of vminserts B know about only vmstorages B and this is explicitly specified via `-storageNode` [configuration](../Cluster-VictoriaMetrics.md#cluster-setup).
1. Groups of vminserts C know about only vmstorages A and this is explicitly specified via `-storageNode` [configuration](../Cluster-VictoriaMetrics.md#cluster-setup).
1. vmselect reads data from all vmstorage nodes via `-storageNode` [configuration](../Cluster-VictoriaMetrics.md#cluster-setup)
   with [deduplication](../Cluster-VictoriaMetrics.md#deduplication) setting equal to vmagent's scrape interval or minimum interval between collected samples.
1. vmagent routes incoming metrics to the given set of `vminsert` nodes using relabeling rules specified at `-remoteWrite.urlRelabelConfig` [configuration](../vmagent.md#relabeling).

**Multi-Tenant Setup**

Every group of vmstorages can handle one tenant or multiple one. Different groups can have overlapping tenants. As vmselect reads from all vmstorage nodes, the data is aggregated on its level.

**Additional Enhancements**

You can set up [vmauth](../vmauth.md) for routing data to the given vminsert group depending on the needed retention.
