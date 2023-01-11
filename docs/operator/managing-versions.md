---
sort: 8
---


# Managing application versions

## VMAlert, VMAgent, VMAlertmanager, VMSingle version


for those objects you can specify following settings at `spec.Image`

for instance, to set `VMSingle` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
cat <<EOF | kubectl apply -f  -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  image:
    repository: victoriametrics/victoria-metrics
    tag: v1.39.2
    pullPolicy: Always
  retentionPeriod: "1"
EOF
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:
```yaml
cat <<EOF | kubectl apply -f  -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  imagePullSecrets:
  - name: my-repo-secret
  image:
    repository: my-repo-url/victoria-metrics
    tag: v1.39.2
  retentionPeriod: "1"
EOF
```


# VMCluster

for `VMCluster` you can specify tag and repository setting per cluster object. 
But `imagePullSecrets` is global setting for all `VMCluster` specification.
```yaml
cat << EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMCluster
metadata:
  name: example-vmcluster
spec:
  imagePullSecrets:
  - name: my-repo-secret
  # Add fields here
  retentionPeriod: "1"
  vmstorage:
      replicaCount: 2
      image:
        repository: victoriametrics/vmstorage
        tag: v1.39.2-cluster
        pullPolicy: Always
  vmselect:
      replicaCount: 2
      image:
        repository: victoriametrics/vmselect
        tag: v1.39.2-cluster
        pullPolicy: Always
  vminsert:
      replicaCount: 2
      image:
        repository: victoriametrics/vminsert
        tag: v1.39.2-cluster
        pullPolicy: Always
EOF
```



