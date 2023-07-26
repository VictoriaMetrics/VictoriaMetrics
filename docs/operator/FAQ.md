---
sort: 15
weight: 15
title: FAQ
menu:
  docs:
    parent: "operator"
    weight: 15
    identifier: "faq-operator"
aliases:
- /operator/FAQ.html
---

# FAQ

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
