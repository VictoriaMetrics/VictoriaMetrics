---
sort: 7
---

# High Availability

High availability is not only important for customer-facing software but if the monitoring infrastructure is not highly available, then there is a risk that operations people are not notified of alerts. Therefore high availability must be just as thought through for the monitoring stack, as for anything else.

## VMAgent

To run VMAgent in a highly available manner you have to configure deduplication at Victoria Metrics first [doc](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/Single-server-VictoriaMetrics.md#deduplication)

Then increase replicas for VMAgent.

create `VMSingle` with dedup flag
```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle-persisted
spec:
  retentionPeriod: "1"
  extraArgs:
    dedup.minScrapeInterval: 60s
EOF
```
create `VMAgent` with 2 replicas
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
  scrapeInterval: 60s
  vmAgentExternalLabelName: vmagent-ha
  replicaCount: 2
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
EOF

```

### Sharding
Sharding for `VMAgent` distributes scraping between multiple deployments of `VMAgent`.
more info https://victoriametrics.github.io/vmagent.html#scraping-big-number-of-targets

Example usage:
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
  scrapeInterval: 60s
  vmAgentExternalLabelName: vmagent-ha
  shardCount: 5
  replicaCount: 2
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
EOF
```

This configuration produces 5 deployments with 2 replicas at each. Each deployment has its own shard num
and scrapes only 1/5 of all targets.

### StatefulMode

In `StatefulMode` `VMAgent` doesn't lose state of the PersistentQueue (file-based buffer size for unsent data) on pod restarts.

Operator creates `StatefulSet` and, with provided `PersistenVolumeClaimTemplate` at `StatefulStorage` configuration param, metrics queue is stored on disk.

Example of configuration:
```yaml 
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-persistent
spec:
  selectAllByDefault: true
  vmAgentExternalLabelName: vmagent-ha
  replicaCount: 2
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
  statefulMode: true
  statefulStorage:
    volumeClaimTemplate:
      spec:
        resources:
            requests:
              storage: 20Gi
```

## VMAlert

It can be launched with multiple replicas without an additional configuration, alertmanager is responsible for alert deduplication.
Note, if you want to use `VMAlert` with high-available `VMAlertmanager`, which has more than 1 replica. You have to specify all pod fqdns
at `VMAlert.spec.notifiers.[url]`. Or you can use service discovery for notifier, examples:

alertmanager:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vmalertmanager-example-alertmanager
  labels:
    app: vm-operator
type: Opaque
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    route:
      group_by: ['job']
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      receiver: 'webhook'
    receivers:
      - name: 'webhook'
        webhook_configs:
          - url: 'http://alertmanagerwh:30500/'

---
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example
  namespace: default
  labels:
   usage: dedicated
spec:
  replicaCount: 2
  configSecret: vmalertmanager-example-alertmanager
  configSelector: {}
  configNamespaceSelector: {}
```
vmalert with fqdns:
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-ha
  namespace: default
spec:
  datasource:
    url: http://vmsingle-example.default.svc:8429
  notifiers:
    - url: http://vmalertmanager-example-0.vmalertmanager-example.default.svc:9093
    - url: http://vmalertmanager-example-1.vmalertmanager-example.default.svc:9093
```

vmalert with service discovery:
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-ha
  namespace: default
spec:
  datasource:
   url: http://vmsingle-example.default.svc:8429
  notifiers:
    - selector:
        namespaceSelector:
          matchNames: 
            - default
        labelSelector:
          matchLabels:
              usage: dedicated
```


## VMSingle

It doesn't support high availability by default, for such purpose use VMCluster or duplicate the setup.


## VMCluster

The cluster version provides a full set of high availability features - metrics replication, node failover, horizontal scaling.

For using the cluster version you have to create the corresponding CRD object:

```yaml
cat << EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMCluster
metadata:
  name: example-vmcluster-persistent
spec:
  retentionPeriod: "4"
  replicationFactor: 2       
  vmstorage:
    replicaCount: 2
    storageDataPath: "/vm-data"
    podMetadata:
      labels:
        owner: infra
    affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: "app.kubernetes.io/name"
                operator: In
                values:
                - "vmstorage"
            topologyKey: "kubernetes.io/hostname"
    storage:
      volumeClaimTemplate:
        spec:
          resources:
            requests:
              storage: 10Gi
    resources:
      limits:
        cpu: "2"
        memory: 2048Mi
  vmselect:
    replicaCount: 2
    cacheMountPath: "/select-cache"
    podMetadata:
      labels:
        owner: infra
    affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: "app.kubernetes.io/name"
                operator: In
                values:
                - "vmselect"
            topologyKey: "kubernetes.io/hostname"

    storage:
      volumeClaimTemplate:
        spec:
          resources:
            requests:
              storage: 2Gi
    resources:
      limits:
        cpu: "1"
        memory: "500Mi"
  vminsert:
    replicaCount: 2
    podMetadata:
      labels:
        owner: infra
    affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: "app.kubernetes.io/name"
                operator: In
                values:
                - "vminsert"
            topologyKey: "kubernetes.io/hostname"
    resources:
      limits:
        cpu: "1"
        memory: "500Mi"

EOF
```

Then wait for the cluster becomes ready

```console
kubectl get vmclusters -w
NAME                           INSERT COUNT   STORAGE COUNT   SELECT COUNT   AGE   STATUS
example-vmcluster-persistent   2              2               2              2s    expanding
example-vmcluster-persistent   2              2               2              30s   operational
```

Get links for connection by executing the command:

```console
kubectl get svc -l app.kubernetes.io/instance=example-vmcluster-persistent
NAME                                     TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)                      AGE
vminsert-example-vmcluster-persistent    ClusterIP   10.96.34.94   <none>        8480/TCP                     69s
vmselect-example-vmcluster-persistent    ClusterIP   None          <none>        8481/TCP                     79s
vmstorage-example-vmcluster-persistent   ClusterIP   None          <none>        8482/TCP,8400/TCP,8401/TCP   85s
```

Now you can connect vmagent to vminsert and vmalert to vmselect

>NOTE do not forget to create rbac for vmagent

```yaml
cat << EOF | kubectl apply -f  -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeNamespaceSelector: {}
  serviceScrapeSelector: {}
  podScrapeNamespaceSelector: {}
  podScrapeSelector: {}
  # Add fields here
  replicaCount: 1
  remoteWrite:
    - url: "http://vminsert-example-vmcluster-persistent.default.svc.cluster.local:8480/insert/0/prometheus/api/v1/write"
EOF
```

Config for vmalert

```yaml
cat << EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-vmalert
spec:
  # Add fields here
  replicas: 1
  datasource:
    url: "http://vmselect-example-vmcluster-persistent.default.svc.cluster.local:8481/select/0/prometheus"
  notifier:
    url: "http://alertmanager-operated.default.svc:9093"
  evaluationInterval: "10s"
  ruleSelector: {}
EOF
```


## Alertmanager

The final step of the high availability scheme is Alertmanager, when an alert triggers, actually fire alerts against *all* instances of an Alertmanager cluster.

The Alertmanager, starting with the `v0.5.0` release, ships with a high availability mode. It implements a gossip protocol to synchronize instances of an Alertmanager cluster regarding notifications that have been sent out, to prevent duplicate notifications. It is an AP (available and partition tolerant) system. Being an AP system means that notifications are guaranteed to be sent at least once.

The Victoria Metrics Operator ensures that Alertmanager clusters are properly configured to run highly available on Kubernetes.
