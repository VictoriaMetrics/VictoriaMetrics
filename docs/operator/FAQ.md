---
sort: 9
weight: 9
title: FAQ
menu:
  docs:
    parent: "operator"
    weight: 9
    identifier: "operator-faq"
aliases:
  - /operator/FAQ.html
  - /operator/faq.html
---

# FAQ (Frequency Asked Questions)

## How do you monitor the operator itself?

You can read about vmoperator monitoring in [this document](./monitoring.md).

## How to change VMStorage PVC storage class

With Helm chart deployment:

1. Update the PVCs manually
1. Run `kubectl delete statefulset --cascade=orphan {vmstorage-sts}` which will delete the sts but keep the pods
1. Update helm chart with the new storage class in the volumeClaimTemplate
1. Run the helm chart to recreate the sts with the updated value

With Operator deployment:

1. Update the PVCs manually
1. Run `kubectl delete vmcluster --cascade=orphan {cluster-name}`
1. Run `kubectl delete statefulset --cascade=orphan {vmstorage-sts}`
1. Update VMCluster spec to use new storage class
1. Apply cluster configuration

## How to override image registry

You can use `VM_CONTAINERREGISTRY` parameter for operator:

- See details about tuning [operator settings here](./setup.md#settings).
- See [available operator settings](./vars.md) here.

## How to set up automatic backups?

You can read about backups:

- for `VMSingle`: [Backup automation](./resources/vmsingle.md#backup-automation)
- for `VMCluster`: [Backup automation](./resources/vmcluster.md#backup-automation)

## How to migrate from Prometheus-operator to VictoriaMetrics operator?

You can read about migration from prometheus operator on [this page](./migration.md).

## How to turn off conversion for prometheus resources

You can read about it on [this page](./migration.md#objects-convesion).

## My VM objects are not deleted/changed when I delete/change Prometheus objects

You can read about it in following sections of "Migration from prometheus-operator" docs:

- [Deletion synchronization](./migration.md#deletion-synchronization)
- [Update synchronization](./migration.md#update-synchronization)
- [Labels synchronization](./migration.md#labels-synchronization)

## What permissions does an operator need to run in a cluster?

You can read about needed permissions for operator in [this document](./security.md#roles).

## How to know the version of VM components in the operator?

See [printDefaults mode](./configuration.md).

In addition, you can use [Release notes](https://github.com/VictoriaMetrics/operator/releases) 
or [CHANGELOG](https://github.com/VictoriaMetrics/operator/blob/master/docs/CHANGELOG.md).
- that's where we describe default version of VictoriaMetrics components.

## How to run VictoriaMetrics operator with permissions for one namespace only?

See this document for details: [Configuration -> Namespaced mode](./configuration.md#namespaced-mode).

## How to configure VMAgent and VMServiceScrape for using with [Istio Service Mesh](https://istio.io/) and its mTLS?

See this example in operator repository: https://github.com/VictoriaMetrics/operator/blob/master/config/examples/vmagent-istio.yaml

## What versions of Kubernetes is the operator compatible with?

Operator tested at kubernetes versions from 1.16 to 1.27.
