---
sort: 10
---

# Relabeling

## VMAgent relabel


`VMAgent` supports global relabeling for all metrics and per remoteWrite target relabel config. 

> Note in some cases, you don't need relabeling,
> key=value label pairs can be added to the all scrapped metrics with `spec.externalLabels` for `VMAgent`.
> 
```yaml
# simple label add config
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
 name: stack
spec:
 externalLabels:
  clusterid: some_cluster
``` 



 It supports relabeling with custom configMap or inline defined at CRD
 
## Configmap example

 Quick tour how to to create `Confimap` with relabeling configuration
 
 ```yaml
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: vmagent-relabel
data:
  global-relabel.yaml: |
        - target_label: bar
        - source_labels: [aa]
          separator: "foobar"
          regex: "foo.+bar"
          target_label: aaa
          replacement: "xxx"
        - action: keep
          source_labels: [aaa]
        - action: drop
          source_labels: [aaa]
  target-1-relabel.yaml: |
        - action: keep_if_equal
          source_labels: [foo, bar]
        - action: drop_if_equal
          source_labels: [foo, bar]
EOF
```

Second, add `relabelConfig` to `VMagent` spec for global relabeling with name of `Configmap` - `vmagent-relabel` and key `global-relabel.yaml`.
 For relabeling per remoteWrite target, add   `urlRelabelConfig` name of `Configmap` - `vmagent-relabel` and key `target-1-relabel.yaml` to one of remoteWrite target for relabeling only
for those target.

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeNamespaceSelector: {}
  podScrapeNamespaceSelector: {}
  podScrapeSelector: {}
  serviceScrapeSelector: {}
  replicaCount: 1
  serviceAccountName: vmagent
  relabelConfig:
   name: "vmagent-relabel"
   key: "global-relabel.yaml"
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
    - url: "http://vmsingle-example-vmsingle.default.svc:8429/api/v1/write"
      urlRelabelConfig:
        name: "vmagent-relabel"
        key: "target-1-relabel.yaml"
EOF
```


## Inline example

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeNamespaceSelector: {}
  podScrapeNamespaceSelector: {}
  podScrapeSelector: {}
  serviceScrapeSelector: {}
  replicaCount: 1
  serviceAccountName: vmagent
  inlineRelabelConfig:
   - target_label: bar
   - source_labels: [aa]
     separator: "foobar"
     regex: "foo.+bar"
     target_label: aaa
     replacement: "xxx"
   - action: keep
     source_labels: [aaa]
   - action: drop
     source_labels: [aaa]
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
    - url: "http://vmsingle-example-vmsingle.default.svc:8429/api/v1/write"
      inlineUrlRelabelConfig:
       - action: keep_if_equal
         source_labels: [foo, bar]
       - action: drop_if_equal
         source_labels: [foo, bar]
EOF
```


##  Combined example

 Its also possible to use both features in combination.

 First will be added relabeling configs from  `inlineRelabelConfig`, then `relabelConfig` from configmap.

 ```yaml
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: vmagent-relabel
data:
  global-relabel.yaml: |
        - target_label: bar
        - source_labels: [aa]
          separator: "foobar"
          regex: "foo.+bar"
          target_label: aaa
          replacement: "xxx"
        - action: keep
          source_labels: [aaa]
        - action: drop
          source_labels: [aaa]
  target-1-relabel.yaml: |
        - action: keep_if_equal
          source_labels: [foo, bar]
        - action: drop_if_equal
          source_labels: [foo, bar]
EOF
```

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeNamespaceSelector: {}
  podScrapeNamespaceSelector: {}
  podScrapeSelector: {}
  serviceScrapeSelector: {}
  replicaCount: 1
  serviceAccountName: vmagent
  inlineRelabelConfig:
   - target_label: bar1
   - source_labels: [aa]
  relabelConfig:
   name: "vmagent-relabel"
   key: "global-relabel.yaml"
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
    - url: "http://vmsingle-example-vmsingle.default.svc:8429/api/v1/write"
      urlRelabelConfig:
        name: "vmagent-relabel"
        key: "target-1-relabel.yaml"
      inlineUrlRelabelConfig:
        - action: keep_if_equal
          source_labels: [foo1, bar2]
EOF
```


 Resulted configmap:
```yaml
apiVersion: v1
data:
  global_relabeling.yaml: |
    - target_label: bar1
    - source_labels:
      - aa
    - target_label: bar
    - source_labels: [aa]
      separator: "foobar"
      regex: "foo.+bar"
      target_label: aaa
      replacement: "xxx"
    - action: keep
      source_labels: [aaa]
    - action: drop
      source_labels: [aaa]
  url_rebaling-1.yaml: |
    - source_labels:
      - foo1
      - bar2
      action: keep_if_equal
    - action: keep_if_equal
      source_labels: [foo, bar]
    - action: drop_if_equal
      source_labels: [foo, bar]
kind: ConfigMap
metadata:
  finalizers:
  - apps.victoriametrics.com/finalizer
  labels:
    app.kubernetes.io/component: monitoring
    app.kubernetes.io/instance: example-vmagent
    app.kubernetes.io/name: vmagent
    managed-by: vm-operator
  name: relabelings-assets-vmagent-example-vmagent
  namespace: default
  ownerReferences:
  - apiVersion: operator.victoriametrics.com/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: VMAgent
    name: example-vmagent
    uid: 7e9fb838-65da-4443-a43b-c00cd6c4db5b
```


## Additional information

`VMAgent` also has some extra options for relabeling actions, you can check it [docs](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmagent/README.md#relabeling)
