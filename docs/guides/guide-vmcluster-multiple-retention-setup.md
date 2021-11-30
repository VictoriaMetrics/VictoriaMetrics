# Multi Retention Setup within VictoriaMetrics Cluster


**Objective**

Setup Victoria Metrics TSDB with support of multiple retention periods within one installation.

**Challenge**

VictoriaMetrics instance (single node or vmstorage node) supports only one retention period.


**Solution**

A multi-retention setup can be implemented by dividing a [victoriametrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) for logical groups. 

Example:
Setup should handle 3 different retention groups 3months, 1year and 3 years.
Solution contains 3 groups of vmstorages + vminserst and one group of vmselects. Routing is done by [vmagent](https://docs.victoriametrics.com/vmagent.html) and [relabeling configuration](https://docs.victoriametrics.com/vmagent.html#relabeling) 

The diagram below shows a proposed solution

<p align="center">
  <img src="guide-vmcluster-multiple-retention-scheme.png" width="800">
</p>

**Implementation Details**
  1. Groups of vminserts A know about only vmstorages A and this is explicitly specified in [storageNode configuration](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#cluster-setup). 
  2. Groups of vminserts B know about only vmstorages B and this is explicitly specified in storageNode configuration. 
  3. Groups of vminserts C know about only vmstorages A and this is explicitly specified in storageNode configuration. 
  4. Vmselect reads data from all vmstorage nodes.
  5. Vmagents divides input data by labels or metrics. Every source can have label retention=X for this purpose and can be drop on [vminsert level](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#list-of-command-line-flags-for-vminsert).

**Multi-Tenant Setup**

Every group of vmstorages can handle one tenant or multiple one. Different groups can have overlapping tenants. As vmselects reads from all storages the data is aggregated on its level. 

**Additional Enhancements**

You can set up additional vmagent per each vminsert+vmstorage group, this allows to separate configuration of routing by retention period and general relabeling rules.
