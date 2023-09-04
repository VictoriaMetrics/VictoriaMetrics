---
sort: 99
weight: 99
title: VictoriaMetrics Enterprise
menu:
  docs:
    parent: "victoriametrics"
    weight: 99
aliases:
- /enterprise.html
---

# VictoriaMetrics Enterprise

VictoriaMetrics components are provided in two kinds - [community edition](https://victoriametrics.com/products/open-source/)
and [enterprise edition](https://victoriametrics.com/products/enterprise/).

VictoriaMetrics community components are open source and are free to use - see [the source code](https://github.com/VictoriaMetrics/VictoriaMetrics/)
and [the license](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE).

The use of VictoriaMetrics enterprise components is permitted in the following cases:

- Evaluation use in non-production setups. Just download and run enterprise binaries or packages of VictoriaMetrics
  components from usual places - [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) and [docker hub](https://hub.docker.com/u/victoriametrics).
  Enterprise binaries and packages have `enterprise` suffix in their names.

- Production use if you have a valid enterprise contract or valid permit from VictoriaMetrics company.
  [Contact us](mailto:info@victoriametrics.com) if you need such contract.

- [Managed VictoriaMetrics](https://docs.victoriametrics.com/managed-victoriametrics/) is built on top of enterprise binaries of VictoriaMetrics.

All the enterprise apps require `-eula` command-line flag to be passed to them. This flag acknowledges that your usage fits one of the cases listed above.

## VictoriaMetrics enterprise features

VictoriaMetrics enterprise includes [all the features of the community edition](https://docs.victoriametrics.com/#prominent-features),
plus the following additional features:

- [Downsampling](https://docs.victoriametrics.com/#downsampling) - this feature allows reducing storage costs
  and increasing performance for queries over historical data.
- [Multiple retentions](https://docs.victoriametrics.com/#retention-filters) - this feature allows reducing storage costs
  by specifying different retentions to different datasets.
- [Automatic discovery of vmstorage nodes](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery) -
  this feature allows updating the list of `vmstorage` nodes at `vminsert` and `vmselect` without the need to restart these services.
- [Backup automation](https://docs.victoriametrics.com/vmbackupmanager.html).
- [Advanced per-tenant stats](https://docs.victoriametrics.com/PerTenantStatistic.html).
- [Advanced auth and rate limiter](https://docs.victoriametrics.com/vmgateway.html).
- [mTLS for cluster components](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection).
- [Kafka integration](https://docs.victoriametrics.com/vmagent.html#kafka-integration).
- [Multitenant support in vmalert](https://docs.victoriametrics.com/vmalert.html#multitenancy).
- [Ability to read alerting and recording rules from object storage](https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage).
- [Ability to filter incoming requests by IP at vmauth](https://docs.victoriametrics.com/vmauth.html#ip-filters).
- [Anomaly Detection Service](https://docs.victoriametrics.com/vmanomaly.html).

On top of this, enterprise package of VictoriaMetrics includes the following important Enterprise features:

- First-class consulting and technical support provided by the core dev team.
- [Monitoring of monitoring](https://victoriametrics.com/products/mom/) - this feature allows forecasting
  and preventing possible issues in VictoriaMetrics setups.
- [Enterprise security compliance](https://victoriametrics.com/security/).
- Prioritizing of feature requests from Enterprise customers.

[Contact us](mailto:info@victoriametrics.com) if you are interested in VictoriaMetrics enterprise.
