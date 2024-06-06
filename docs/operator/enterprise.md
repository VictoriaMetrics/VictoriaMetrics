---
sort: 13
weight: 13
title: Enterprise features
menu:
  docs:
    parent: "operator"
    weight: 13
aliases:
  - /operator/enterprise.html
---

# Using operator with enterprise features 

Operator doesn't have enterprise version for itself, but it supports 
[enterprise features for VictoriaMetrics components](https://docs.victoriametrics.com/enterprise.html):

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
  - [Backup atomation](./resources/vmcluster.md#backup-atomation)
- [VMRule Enterprise features](./resources/vmrule.md#enterprise-features)
  - [Multitenancy](./resources/vmrule.md#multitenancy)
- [VMSingle Enterprise features](./resources/vmsingle.md#enterprise-features)
  - [Downsampling](./resources/vmsingle.md#downsampling)
  - [Retention filters](./resources/vmsingle.md#retention-filters)
  - [Backup atomation](./resources/vmsingle.md#backup-atomation)
- [VMUser Enterprise features](./resources/vmuser.md#enterprise-features)
  - [IP Filters](./resources/vmuser.md#ip-filters) 

More information about enterprise features you can read 
on [VictoriaMetrics Enterprise page](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise-features).

In order to find examples of deploying enterprise components with operator,
please, check [this](https://docs.victoriametrics.com/enterprise.html#kubernetes-operator) documentation.
