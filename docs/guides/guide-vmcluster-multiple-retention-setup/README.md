---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
**Objective**

Setup Victoria Metrics Cluster with support of multiple retention periods within one installation.

**Enterprise Solution**

[VictoriaMetrics Enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/) supports multiple retention periods natively on both the [cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#retention-filters) and the [single node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#multiple-retentions) versions.
You can filter which metrics a retention filter applies to. Below you can see 3 retention filters. The first one matches any metrics with the `juniours` tag and will keep those for 3 days. The second filter says anything with `dev` or `staging` should be kept for 30 days. And finally, the last filter is the default filter of 1 year.
```bash
-retentionFilter='{team="juniors"}:3d' -retentionFilter='{env=~"dev|staging"}:30d' -retentionPeriod=1y
```

When you run the cluster version, you can also set retention filters by tenant ID. Below is a retention filter that will keep metrics from tenant 5 for only 5 days while keeping everyone else's for 1 year. You can also combine this with tags to get even finer control.
```bash
-retentionFilter='{vm_account_id="5"}:5d' -retentionPeriod=1y
```

**Open Source Solution**

Community version of VictoriaMetrics supports only one retention period per `vmstorage` node via [-retentionPeriod](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention) command-line flag.

A multi-retention setup can be implemented by dividing a [victoriametrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) into logical groups with different retentions.

Example:
Setup should handle 3 different retention groups 3months, 1year and 3 years.
Solution contains 3 groups of vmstorages + vminserts and one group of vmselects. Routing is done by [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/)
by [splitting data streams](https://docs.victoriametrics.com/victoriametrics/vmagent/#splitting-data-streams-among-multiple-systems). 
The [-retentionPeriod](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention) sets how long to keep the metrics.

The diagram below shows a proposed solution

![Setup](setup.webp)

**Implementation Details**

1. Groups of vminserts A know about only vmstorages A and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup). 
1. Groups of vminserts B know about only vmstorages B and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup). 
1. Groups of vminserts C know about only vmstorages C and this is explicitly specified via `-storageNode` [configuration](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup). 
1. vmselect reads data from all vmstorage nodes via `-storageNode` [configuration](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup) 
   with [deduplication](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#deduplication) setting equal to vmagent's scrape interval or minimum interval between collected samples. 
1. vmagent routes incoming metrics to the given set of `vminsert` nodes using relabeling rules specified at `-remoteWrite.urlRelabelConfig` [configuration](https://docs.victoriametrics.com/victoriametrics/relabeling/).

**Multi-Tenant Setup**

Every group of vmstorages can handle one tenant or multiple one. Different groups can have overlapping tenants. As vmselect reads from all vmstorage nodes, the data is aggregated on its level.

**Additional Enhancements**

You can set up [vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/) for routing data to the given vminsert group depending on the needed retention.

**Downsides Of This Approach**

In the approach shown above, you have a separate group of nodes for each retention period. Each storage node needs to have a copy of the index, and
more nodes means more indexes, which means more storage space devoted to data that isn't your metrics. With the [Enterprise version](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#retention-filters) one node can handle multiple retention periods, reducing the number of nodes and thus the storage space taken up by indexes.
The index can be quite large on systems where they have time series that change frequently. In some cases, the index size can be larger than the space you're saving with separate retention periods.

Configuration complexity is also a concern; each retention period would have its own storage nodes and unique configurations.
Networking is also more complex; each retention period has its own write path, increasing network complexity.