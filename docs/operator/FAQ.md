---
sort: 15
---

# FAQ

## How to change VMStorage PVC storage class

With Helm chart deployment:

1. Update the PVCs manually
2. Run `kubectl delete statefulset --cascade=orphan {vmstorage-sts}` which will delete the sts but keep the pods
3. Update helm chart with the new storage class in the volumeClaimTemplate
4. Run the helm chart to recreate the sts with the updated value

With Operator deployment:

1. Update the PVCs manually
2. Run `kubectl delete vmcluster --cascade=orphan {cluster-name}`
3. Run `kubectl delete statefulset --cascade=orphan {vmstorage-sts}`
4. Update VMCluster spec to use new storage class
5. Apply cluster configuration
