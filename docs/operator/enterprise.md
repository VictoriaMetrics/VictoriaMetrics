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
[enterprise features for VictoriaMetrics components](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs/enterprise.md):

- [VMAgent Enterprise features](./resources/vmagent.md#enterprise-features):
  - [Reading metrics from kafka](./resources/vmagent.md#reading-metrics-from-kafka)
  - [Writing metrics to kafka](./resources/vmagent.md#writing-metrics-to-kafka)
- [VMAlert Enterprise features](./resources/vmalert.md#enterprise-features):
  - [Reading rules from object storage](./resources/vmalert.md#reading-rules-from-object-storage)
  - [Multitenancy](./resources/vmalert.md#multitenancy)
- [VMAuth Enterprise features](./resources/vmauth.md#enterprise-features)
  - [IP Filters](./resources/vmauth.md#ip-filters) 
- [VMCluster Enterprise features](./resources/vmcluster.md#enterprise-features)
  - [Downsampling](./resources/vmcluster.md#downsampling)
  - [Multiple retentions / Retention filters](./resources/vmcluster.md#retention-filters)
  - [Advanced per-tenant statistic](./resources/vmcluster.md#advanced-per-tenant-statistic)
  - [mTLS protection](./resources/vmcluster.md#mtls-protection)
  - [Backup automation](./resources/vmcluster.md#backup-automation)
- [VMRule Enterprise features](./resources/vmrule.md#enterprise-features)
  - [Multitenancy](./resources/vmrule.md#multitenancy)
- [VMSingle Enterprise features](./resources/vmsingle.md#enterprise-features)
  - [Downsampling](./resources/vmsingle.md#downsampling)
  - [Retention filters](./resources/vmsingle.md#retention-filters)
  - [Backup automation](./resources/vmsingle.md#backup-automation)
- [VMUser Enterprise features](./resources/vmuser.md#enterprise-features)
  - [IP Filters](./resources/vmuser.md#ip-filters) 

More information about enterprise features you can read 
on [VictoriaMetrics Enterprise page](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs/enterprise.md#victoriametrics-enterprise-features).

In order to find examples of deploying enterprise components with operator,
please, check [this](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs/enterprise.md#kubernetes-operator) documentation.
