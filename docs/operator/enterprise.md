---
weight: 13
title: Enterprise features
menu:
  docs:
    parent: "operator"
    weight: 13
aliases:
  - /operator/enterprise/
  - /operator/enterprise/index.html
---
Operator doesn't have enterprise version for itself, but it supports 
[enterprise features for VictoriaMetrics components](https://docs.victoriametrics.com/enterprise/):

- [VMAgent Enterprise features](https://docs.victoriametrics.com/operator/resources/vmagent/#enterprise-features):
  - [Reading metrics from kafka](https://docs.victoriametrics.com/operator/resources/vmagent/#reading-metrics-from-kafka)
  - [Writing metrics to kafka](https://docs.victoriametrics.com/operator/resources/vmagent/#writing-metrics-to-kafka)
- [VMAlert Enterprise features](https://docs.victoriametrics.com/operator/resources/vmalert/#enterprise-features):
  - [Reading rules from object storage](https://docs.victoriametrics.com/operator/resources/vmalert/#reading-rules-from-object-storage)
  - [Multitenancy](https://docs.victoriametrics.com/operator/resources/vmalert/#multitenancy)
- [VMAuth Enterprise features](https://docs.victoriametrics.com/operator/resources/vmauth/#enterprise-features)
  - [IP Filters](https://docs.victoriametrics.com/operator/resources/vmauth/#ip-filters) 
- [VMCluster Enterprise features](https://docs.victoriametrics.com/operator/resources/vmcluster/#enterprise-features)
  - [Downsampling](https://docs.victoriametrics.com/operator/resources/vmcluster/#downsampling)
  - [Multiple retentions / Retention filters](https://docs.victoriametrics.com/operator/resources/vmcluster/#retention-filters)
  - [Advanced per-tenant statistic](https://docs.victoriametrics.com/operator/resources/vmcluster/#advanced-per-tenant-statistic)
  - [mTLS protection](https://docs.victoriametrics.com/operator/resources/vmcluster/#mtls-protection)
  - [Backup automation](https://docs.victoriametrics.com/operator/resources/vmcluster/#backup-automation)
- [VMRule Enterprise features](https://docs.victoriametrics.com/operator/resources/vmrule/#enterprise-features)
  - [Multitenancy](https://docs.victoriametrics.com/operator/resources/vmrule/#multitenancy)
- [VMSingle Enterprise features](https://docs.victoriametrics.com/operator/resources/vmsingle/#enterprise-features)
  - [Downsampling](https://docs.victoriametrics.com/operator/resources/vmsingle/#downsampling)
  - [Retention filters](https://docs.victoriametrics.com/operator/resources/vmsingle/#retention-filters)
  - [Backup automation](https://docs.victoriametrics.com/operator/resources/vmsingle/#backup-automation)
- [VMUser Enterprise features](https://docs.victoriametrics.com/operator/resources/vmuser/#enterprise-features)
  - [IP Filters](https://docs.victoriametrics.com/operator/resources/vmuser/#ip-filters) 

More information about enterprise features you can read 
on [VictoriaMetrics Enterprise page](https://docs.victoriametrics.com/enterprise#victoriametrics-enterprise-features).

In order to find examples of deploying enterprise components with operator,
please, check [this](https://docs.victoriametrics.com/enterprise#kubernetes-operator) documentation.
