---
sort: 1
weight: 1
title: VMAgent
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 1
aliases:
  - /operator/resources/vmagent.html
---

# VMAgent

`VMAgent` represents agent, which helps you collect metrics from various sources and stores them in VictoriaMetrics.
The `VMAgent` CRD declaratively defines a desired [VMAgent](https://docs.victoriametrics.com/vmagent)
setup to run in a Kubernetes cluster.

It requires access to Kubernetes API and you can create RBAC for it first, it can be found 
at [`examples/vmagent_rbac.yaml`](https://github.com/VictoriaMetrics/operator/blob/master/config/examples/vmagent_rbac.yaml)
Or you can use default rbac account, that will be created for `VMAgent` by operator automatically.

For each `VMAgent` resource Operator deploys a properly configured `Deployment` in the same namespace.
The VMAgent `Pod`s are configured to mount a `Secret` prefixed with `<VMAgent-name>` containing the configuration
for VMAgent.

For each `VMAgent` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with
name `<VMAgent-name>`.

The CRD specifies which `VMServiceScrape` should be covered by the deployed VMAgent instances based on label selection.
The Operator then generates a configuration based on the included `VMServiceScrape`s and updates the `Secret` which
contains the configuration. It continuously does so for all changes that are made to the `VMServiceScrape`s or the
`VMAgent` resource itself.

If no selection of `VMServiceScrape`s is provided - Operator leaves management of the `Secret` to the user,
so user can set custom configuration while still benefiting from the Operator's capabilities of managing VMAgent setups.

## Specification

You can see the full actual specification of the `VMAgent` resource in the **[API docs -> VMAgent](../api.md#vmagent)**.

If you can't find necessary field in the specification of the custom resource, 
see [Extra arguments section](./README.md#extra-arguments).

Also, you can check out the [examples](#examples) section.

## Scraping

`VMAgent` supports scraping targets with:

- [VMServiceScrape](./vmservicescrape.md),
- [VMPodScrape](./vmpodscrape.md),
- [VMNodeScrape](./vmnodescrape.md),
- [VMStaticScrape](./vmstaticscrape.md),
- [VMProbe](./vmprobe.md).
- [VMScrapeConfig](./vmscrapeconfig.md)

These objects tell VMAgent from which targets and how to collect metrics and 
generate part of [VMAgent](./vmagent.md) scrape configuration.

For filtering scrape objects `VMAgent` uses selectors. 
Selectors are defined with suffixes - `NamespaceSelector` and `Selector` for each type of scrape objects in spec of `VMAgent`:

- `serviceScrapeNamespaceSelector` and `serviceScrapeSelector` for selecting [VMServiceScrape](./vmservicescrape.md) objects,
- `podScrapeNamespaceSelector` and `podScrapeSelector` for selecting [VMPodScrape](./vmpodscrape.md) objects,
- `probeNamespaceSelector` and `probeSelector` for selecting [VMProbe](./vmprobe.md) objects,
- `staticScrapeNamespaceSelector` and `staticScrapeSelector` for selecting [VMStaticScrape](./vmstaticscrape.md) objects,
- `nodeScrapeNamespaceSelector` and `nodeScrapeSelector` for selecting [VMNodeScrape](./vmnodescrape.md) objects.
- `scrapeConfigNamespaceSelector` and `scrapeConfigSelector` for selecting [VMScrapeConfig](./vmscrapeconfig.md) objects.

It allows configuring objects access control across namespaces and different environments. 
Specification of selectors you can see in [this doc](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta).

In addition to the above selectors, the filtering of objects in a cluster is affected by the field `selectAllByDefault` of `VMAgent` spec and environment variable `WATCH_NAMESPACE` for operator.

Following rules are applied:

- If `...NamespaceSelector` and `...Selector` both undefined, then by default select nothing. With option set - `spec.selectAllByDefault: true`, select all objects of given type.
- If `...NamespaceSelector` defined, `...Selector` undefined, then all objects are matching at namespaces for given `...NamespaceSelector`.
- If `...NamespaceSelector` undefined, `...Selector` defined, then all objects at `VMAgent`'s namespaces are matching for given `...Selector`.
- If `...NamespaceSelector` and `...Selector` both defined, then only objects at namespaces matched `...NamespaceSelector` for given `...Selector` are matching.

Here's a more visual and more detailed view:

| `...NamespaceSelector` | `...Selector` | `selectAllByDefault` | `WATCH_NAMESPACE` | Selected objects                                                                                            |
|------------------------|---------------|----------------------|-------------------|-------------------------------------------------------------------------------------------------------------|
| undefined              | undefined     | false                | undefined         | nothing                                                                                                     |
| undefined              | undefined     | **true**             | undefined         | all objects of given type (`...`) in the cluster                                                            |
| **defined**            | undefined     | *any*                | undefined         | all objects of given type (`...`) at namespaces for given `...NamespaceSelector`                            |
| undefined              | **defined**   | *any*                | undefined         | all objects of given type (`...`) only at `VMAgent`'s namespace are matching for given `Selector            |
| **defined**            | **defined**   | *any*                | undefined         | all objects of given type (`...`) only at namespaces matched `...NamespaceSelector` for given `...Selector` |
| *any*                  | undefined     | *any*                | **defined**       | all objects of given type (`...`) only at `VMAgent`'s namespace                                             |
| *any*                  | **defined**   | *any*                | **defined**       | all objects of given type (`...`) only at `VMAgent`'s namespace for given `...Selector`                     |

More details about `WATCH_NAMESPACE` variable you can read in [this doc](../configuration.md#namespaced-mode).

Here are some examples of `VMAgent` configuration with selectors:

```yaml
# select all scrape objects in the cluster
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-select-all
spec:
  # ...
  selectAllByDefault: true

---

# select all scrape objects in specific namespace (my-namespace)
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-select-ns
spec:
  # ...
  serviceScrapeNamespaceSelector: 
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
  podScrapeNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
  nodeScrapeNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
  staticScrapeNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
  probeNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
  scrapeConfigNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
```

## High availability

<!-- TODO: health checks -->

### Replication and deduplication

To run VMAgent in a highly available manner at first you have to configure deduplication in Victoria Metrics
according [this doc for VMSingle](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#deduplication)
or [this doc for VMCluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#deduplication).

You can do it with `extraArgs` on [`VMSingle`](./vmsingle.md):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: vmsingle-example
spec:
  # ...
  extraArgs:
    dedup.minScrapeInterval: 30s
  # ...
```

For [`VMCluster`](./vmcluster.md) you can do it with `vmstorage.extraArgs` and `vmselect.extraArgs`:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMCluster
metadata:
  name: vmcluster-example
spec:
  # ...
  vmselect:
    extraArgs:
      dedup.minScrapeInterval: 30s
    # ...
  vmstorage:
    extraArgs:
      dedup.minScrapeInterval: 30s
    # ...
```

Deduplication is automatically enabled with `replicationFactor > 1` on `VMCLuster`.

After enabling deduplication you can increase replicas for VMAgent. 

For instance, let's create `VMAgent` with 2 replicas:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-ha-example
spec:
  # ...
  selectAllByDefault: true
  vmAgentExternalLabelName: vmagent_ha
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8429/api/v1/write"
  # Replication:
  scrapeInterval: 30s
  replicaCount: 2
  # ...
```

Now, even if something happens to one of the vmagent, you'll still have the data.

### StatefulMode

VMAgent supports [persistent buffering](https://docs.victoriametrics.com/vmagent.html#replication-and-high-availability)
for sending data to remote storage. By default, operator set `-remoteWrite.tmpDataPath` for `VMAgent` to `/tmp` (that use k8s ephemeral storage)
and `VMAgent` loses state of the PersistentQueue on pod restarts.

In `StatefulMode` `VMAgent` doesn't lose state of the PersistentQueue (file-based buffer size for unsent data) on pod restarts.
Operator creates `StatefulSet` and, with provided `PersistentVolumeClaimTemplate` at `StatefulStorage` configuration param, metrics queue is stored on disk.

Example of configuration for `StatefulMode`:

```yaml 
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-ha-example
spec:
  # ...
  selectAllByDefault: true
  vmAgentExternalLabelName: vmagent_ha
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8429/api/v1/write"
  # Replication:
  scrapeInterval: 30s
  replicaCount: 2
  # StatefulMode:
  statefulMode: true
  statefulStorage:
    volumeClaimTemplate:
      spec:
        resources:
            requests:
              storage: 20Gi
  # ...
```

### Sharding

Operator supports sharding with [cluster mode of vmagent](https://docs.victoriametrics.com/vmagent.html#scraping-big-number-of-targets)
for **scraping big number of targets**.

Sharding for `VMAgent` distributes scraping between multiple deployments of `VMAgent`.

Example usage (it is a complete example of `VMAgent` with high availability features):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-ha-example
spec:
  # ...
  selectAllByDefault: true
  vmAgentExternalLabelName: vmagent_ha
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8429/api/v1/write"
  # Replication:
  scrapeInterval: 30s
  replicaCount: 2
  # StatefulMode:
  statefulMode: true
  statefulStorage:
    volumeClaimTemplate:
      spec:
        resources:
          requests:
            storage: 20Gi
  # Sharding
  shardCount: 5
  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - podAffinityTerm:
            labelSelector:
              matchLabels:
                shard-num: '%SHARD_NUM%'
            topologyKey: kubernetes.io/hostname
  # ...
```

This configuration produces `5` deployments with `2` replicas at each. 
Each deployment has its own shard num and scrapes only `1/5` of all targets.

Also, you can use special placeholder `%SHARD_NUM%` in fields of `VMAgent` specification
and operator will replace it with current shard num of vmagent when creating deployment or statefullset for vmagent.

In the example above, the `%SHARD_NUM%` placeholder is used in the `podAntiAffinity` section,
which recommend to scheduler that pods with the same shard num (label `shard-num` in the pod template)
are not deployed on the same node. You can use another `topologyKey` for availability zone or region instead of nodes. 

**Note** that at the moment operator doesn't use `-promscrape.cluster.replicationFactor` parameter of `VMAgent` and 
creates `replicaCount` of replicas for each shard (which leads greater resource consumption). 
This will be fixed in the future, more details can be seen in [this issue](https://github.com/VictoriaMetrics/operator/issues/604).

Also see [this example](https://github.com/VictoriaMetrics/operator/blob/master/config/examples/vmagent_stateful_with_sharding.yaml).

## Additional scrape configuration

AdditionalScrapeConfigs is an additional way to add scrape targets in `VMAgent` CRD.

There are two options for adding targets into `VMAgent`:

- [inline configuration into CRD](#inline-additional-scrape-configuration-in-vmagent-crd),
- [defining it as a Kubernetes Secret](#define-additional-scrape-configuration-as-a-kubernetes-secret).

No validation happens during the creation of configuration. However, you must validate job specs, and it must follow job spec configuration.
Please check [scrape_configs documentation](https://docs.victoriametrics.com/sd_configs.html#scrape_configs) as references.

### Inline Additional Scrape Configuration in VMAgent CRD

You need to add scrape configuration directly to the `vmagent spec.inlineScrapeConfig`. It is raw text in YAML format.
See example below

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  # ...
  selectAllByDefault: true
  inlineScrapeConfig: |
    - job_name: "prometheus"
      static_configs:
      - targets: ["localhost:9090"]
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8429/api/v1/write"
  # ...
```

**Note**: Do not use passwords and tokens with inlineScrapeConfig use Secret instead.

## Define Additional Scrape Configuration as a Kubernetes Secret

You need to define Kubernetes Secret with a key.

The key is `prometheus-additional.yaml` in the example below:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: additional-scrape-configs
stringData:
  prometheus-additional.yaml: |
    - job_name: "prometheus"
      static_configs:
      - targets: ["localhost:9090"]
```

After that, you need to specify the secret's name and key in VMAgent CRD in `additionalScrapeConfigs` section:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  # ...
  selectAllByDefault: true
  additionalScrapeConfigs:
    name: additional-scrape-configs
    key: prometheus-additional.yaml
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8429/api/v1/write"
  # ...
```

**Note**: You can specify only one Secret in the VMAgent CRD configuration so use it for all additional scrape configurations.

## Relabeling

`VMAgent` supports global relabeling for all metrics and per remoteWrite target relabel config.

Note in some cases, you don't need relabeling, `key=value` label pairs can be added to the all scrapped metrics with `spec.externalLabels` for `VMAgent`:

```yaml
# simple label add config
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  externalLabels:
    clusterid: some_cluster
```

`VMAgent` CR supports relabeling with [custom configMap](#relabeling-config-in-configmap) 
or [inline defined at CRD](#inline-relabeling-config).

### Relabeling config in Configmap

Quick tour how to create `ConfigMap` with relabeling configuration:

 ```yaml
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
```

Second, add `relabelConfig` to `VMagent` spec for global relabeling with name of `Configmap` - `vmagent-relabel` and key `global-relabel.yaml`.

For relabeling per remoteWrite target, add   `urlRelabelConfig` name of `Configmap` - `vmagent-relabel` 
and key `target-1-relabel.yaml` to one of remoteWrite target for relabeling only for those target:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  # ...
  selectAllByDefault: true
  relabelConfig:
   name: "vmagent-relabel"
   key: "global-relabel.yaml"
  remoteWrite:
    - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
    - url: "http://vmsingle-example-vmsingle.default.svc:8429/api/v1/write"
      urlRelabelConfig:
        name: "vmagent-relabel"
        key: "target-1-relabel.yaml"
```

### Inline relabeling config

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  # ...
  selectAllByDefault: true
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
```

###  Combined example

It's also possible to use both features in combination.

First will be added relabeling configs from  `inlineRelabelConfig`, then `relabelConfig` from configmap.

 ```yaml
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
```

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  # ...
  selectAllByDefault: true
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
```

Resulted configmap, mounted to `VMAgent` pod:

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

### Additional information

`VMAgent` also has some extra options for relabeling actions, you can check it [docs](https://docs.victoriametrics.com/vmagent#relabeling).

## Version management

To set `VMAgent` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  image:
    repository: victoriametrics/vmagent
    tag: v1.93.4
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  image:
    repository: victoriametrics/vmagent
    tag: v1.93.4
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VMAgent` resource in the `spec` section of the `VMAgent` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-resources-example
spec:
    # ...
    resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
    # ...
```

If these parameters are not specified, then, 
by default all `VMAgent` pods have resource requests and limits from the default values of the following [operator parameters](../configuration.md):

- `VM_VMAGENTDEFAULT_RESOURCE_LIMIT_MEM` - default memory limit for `VMAgent` pods,
- `VM_VMAGENTDEFAULT_RESOURCE_LIMIT_CPU` - default memory limit for `VMAgent` pods,
- `VM_VMAGENTDEFAULT_RESOURCE_REQUEST_MEM` - default memory limit for `VMAgent` pods,
- `VM_VMAGENTDEFAULT_RESOURCE_REQUEST_CPU` - default memory limit for `VMAgent` pods.

These default parameters will be used if:

- `VM_VMAGENTDEFAULT_USEDEFAULTRESOURCES` is set to `true` (default value), 
- `VMAgent` CR doesn't have `resources` field in `spec` section.

Field `resources` in vmagent spec have higher priority than operator parameters.

If you set `VM_VMAGENTDEFAULT_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VMAgent` CRD,
then `VMAgent` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Enterprise features

VMAgent supports feature [Kafka integration](https://docs.victoriametrics.com/vmagent.html#kafka-integration)
from [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).

For using Enterprise version of [vmagent](https://docs.victoriametrics.com/vmagent.html)
you need to change version of `vmagent` to version with `-enterprise` suffix using [Version management](#version-management).

All the enterprise apps require `-eula` command-line flag to be passed to them.
This flag acknowledges that your usage fits one of the cases listed on [this page](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).
So you can use [extraArgs](./README.md#extra-arguments) for passing this flag to `VMAgent`:

After that you can pass [Kafka integration](https://docs.victoriametrics.com/vmagent.html#kafka-integration) 
flags to `VMAgent` with [extraArgs](./README.md#extra-arguments).

### Reading metrics from Kafka

Here are complete example for [Reading metrics from Kafka](https://docs.victoriametrics.com/vmagent.html#reading-metrics-from-kafka):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmagent
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmagent enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
    
    # using enterprise features: reading metrics from kafka
    # more details about kafka integration you can read on https://docs.victoriametrics.com/vmagent.html#kafka-integration
    # more details about these and other flags you can read on https://docs.victoriametrics.com/vmagent.html#command-line-flags-for-kafka-consumer
    kafka.consumer.topic.brokers: localhost:9092
    kafka.consumer.topic.format: influx
    kafka.consumer.topic: metrics-by-telegraf
    kafka.consumer.topic.groupID: some-id
    
  # ...other fields...
```

### Writing metrics to Kafka

Here are complete example for [Writing metrics to Kafka](https://docs.victoriametrics.com/vmagent.html#writing-metrics-to-kafka):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmagent
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmagent enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
  
  # using enterprise features: writing metrics to Kafka
  # more details about kafka integration you can read on https://docs.victoriametrics.com/vmagent.html#kafka-integration
  remoteWrite:
    # sasl with username and password
    - url: kafka://broker-1:9092/?topic=prom-rw-1&security.protocol=SASL_SSL&sasl.mechanisms=PLAIN 
      # it requires to create kubernetes secret `kafka-basic-auth` with keys `username` and `password` in the same namespace
      basicAuth:
        username:
            name: kafka-basic-auth
            key: username
        password:
            name: kafka-basic-auth
            key: password
    # sasl with username and password from secret and tls
    - url: kafka://localhost:9092/?topic=prom-rw-2&security.protocol=SSL
      # it requires to create kubernetes secret `kafka-tls` with keys `ca.pem`, `cert.pem` and `key.pem` in the same namespace
      tlsConfig:
        ca:
          secret:
            name: kafka-tls
            key: ca.pem
        cert:
          secret:
            name: kafka-tls
            key: cert.pem
        keySecret:
          name: kafka-tls
          key: key.pem

  # ...other fields...
```

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: vmagent-example
spec:
  selectAllByDefault: true
  replicaCount: 1
  scrapeInterval: 30s
  scrapeTimeout: 10s
  vmAgentExternalLabelName: example
  externalLabels:
    cluster: my-cluster
  remoteWrite:
    - url: "http://vmsingle-example.default.svc:8428/api/v1/write"
  inlineRelabelConfig:
    - action: labeldrop
      regex: "temp.*"
```
