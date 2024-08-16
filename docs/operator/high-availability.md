---
weight: 8
title: High Availability
menu:
  docs:
    parent: "operator"
    weight: 8
aliases:
  - /operator/high-availability/
  - /operator/high-availability/index.html
---
High availability is not only important for customer-facing software but if the monitoring infrastructure is not highly available, then there is a risk that operations people are not notified of alerts.
Therefore, high availability must be just as thought through for the monitoring stack, as for anything else.

## Components

VictoriaMetrics operator support high availability for each component of the monitoring stack:

- [VMAgent](https://docs.victoriametrics.com/operator/resources/vmagent/#high-availability)
- [VMAlert](https://docs.victoriametrics.com/operator/resources/vmalert/#high-availability)
- [VMAlertmanager](https://docs.victoriametrics.com/operator/resources/vmalertmanager/#high-availability)
- [VMAuth](https://docs.victoriametrics.com/operator/resources/vmauth/#high-availability)
- [VMCluster](https://docs.victoriametrics.com/operator/resources/vmcluster/#high-availability)

More details you can find in the section **[High Availability for resources](https://docs.victoriametrics.com/operator/resources/#high-availability)**.

## Operator

VictoriaMetrics operator can be safely scaled horizontally, but only one replica of the operator can 
process [the reconciliation](https://docs.victoriametrics.com/operator/#reconciliation-cycle) at a time -
it uses a leader election mechanism to ensure that only one replica is active at a time.

If one of replicas of the operator will be failed, then another replica will be elected as a leader and will continue to work -
operator replication affects how quickly this happens.

[CRD validation](https://docs.victoriametrics.com/operator/configuration#crd-validation) workload is fully 
distributed among the available operator replicas.

In addition, you can safely use for operator such features 
as [assigning and distributing to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)
(like [node selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector), 
[affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity),
[topology spread constraints](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#pod-topology-spread-constraints),
[taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/), etc...)

In addition, don't forget about [monitoring for the operator](https://docs.victoriametrics.com/operator/monitoring/).
