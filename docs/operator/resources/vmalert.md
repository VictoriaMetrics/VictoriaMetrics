---
sort: 2
weight: 2
title: VMAlert
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 2
aliases:
  - /operator/resources/vmalert.html
---

# VMAlert

`VMAlert` - executes a list of given [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) 
or [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) rules against configured address. 

The `VMAlert` CRD declaratively defines a desired [VMAlert](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmalert)
setup to run in a Kubernetes cluster.

It has few required config options - `datasource` and `notifier` are required, for other config parameters
check [doc](../api.md#vmalert).

For each `VMAlert` resource, the Operator deploys a properly configured `Deployment` in the same namespace.
The VMAlert `Pod`s are configured to mount a list of `Configmaps` prefixed with `<VMAlert-name>-number` containing
the configuration for alerting rules.

For each `VMAlert` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with
name `<VMAlert-name>`.

## Specification

You can see the full actual specification of the `VMAlert` resource in the **[API docs -> VMAlert](../api.md#vmalert)**.

If you can't find necessary field in the specification of the custom resource,
see [Extra arguments section](./README.md#extra-arguments).

Also, you can check out the [examples](#examples) section.

## Rules

The CRD specifies which `VMRule`s should be covered by the deployed `VMAlert` instances based on label selection.
The Operator then generates a configuration based on the included `VMRule`s and updates the `Configmaps` containing
the configuration. It continuously does so for all changes that are made to `VMRule`s or to the `VMAlert` resource itself.

Alerting rules are filtered by selectors `ruleNamespaceSelector` and `ruleSelector` in `VMAlert` CRD definition. 
For selecting rules from all namespaces you must specify it to empty value:

```yaml
spec:
  ruleNamespaceSelector: {}
```

[VMRule](./vmrule.md) objects generate part of `VMAlert` configuration.

For filtering rules `VMAlert` uses selectors `ruleNamespaceSelector` and `ruleSelector`.
It allows configuring rules access control across namespaces and different environments.
Specification of selectors you can see in [this doc](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta).

In addition to the above selectors, the filtering of objects in a cluster is affected by the field `selectAllByDefault` of `VMAlert` spec and environment variable `WATCH_NAMESPACE` for operator.

Following rules are applied:

- If `ruleNamespaceSelector` and `ruleSelector` both undefined, then by default select nothing. With option set - `spec.selectAllByDefault: true`, select all vmrules.
- If `ruleNamespaceSelector` defined, `ruleSelector` undefined, then all vmrules are matching at namespaces for given `ruleNamespaceSelector`.
- If `ruleNamespaceSelector` undefined, `ruleSelector` defined, then all vmrules at `VMAlert`'s namespaces are matching for given `ruleSelector`.
- If `ruleNamespaceSelector` and `ruleSelector` both defined, then only vmrules at namespaces matched `ruleNamespaceSelector` for given `ruleSelector` are matching.

Here's a more visual and more detailed view:

| `ruleNamespaceSelector` | `ruleSelector` | `selectAllByDefault` | `WATCH_NAMESPACE` | Selected rules                                                                                       |
|-------------------------|----------------|----------------------|-------------------|------------------------------------------------------------------------------------------------------|
| undefined               | undefined      | false                | undefined         | nothing                                                                                              |
| undefined               | undefined      | **true**             | undefined         | all vmrules in the cluster                                                                           |
| **defined**             | undefined      | *any*                | undefined         | all vmrules are matching at namespaces for given `ruleNamespaceSelector`                             |
| undefined               | **defined**    | *any*                | undefined         | all vmrules only at `VMAlert`'s namespace are matching for given `ruleSelector`                      |
| **defined**             | **defined**    | *any*                | undefined         | all vmrules only at namespaces matched `ruleNamespaceSelector` for given `ruleSelector` are matching |
| *any*                   | undefined      | *any*                | **defined**       | all vmrules only at `VMAlert`'s namespace                                                            |
| *any*                   | **defined**    | *any*                | **defined**       | all vmrules only at `VMAlert`'s namespace for given `ruleSelector` are matching                      |

More details about `WATCH_NAMESPACE` variable you can read in [this doc](../configuration.md#namespaced-mode).

Here are some examples of `VMAlert` configuration with selectors:

```yaml
# select all rule objects in the cluster
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: vmalert-select-all
spec:
  # ...
  selectAllByDefault: true

---

# select all rule objects in specific namespace (my-namespace)
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: vmalert-select-ns
spec:
  # ...
  ruleNamespaceSelector: 
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
```

## High availability

`VMAlert` can be launched with multiple replicas without an additional configuration as far [alertmanager](./vmalertmanager.md) is responsible for alert deduplication.

Note, if you want to use `VMAlert` with high-available [`VMAlertmanager`](./vmalertmanager.md), which has more than 1 replica. 
You have to specify all pod fqdns  at `VMAlert.spec.notifiers.[url]`. Or you can use service discovery for notifier, examples:

- alertmanager:
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
      # ...
    
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
      # ...
    ```
- vmalert with fqdns:
    ```yaml
    apiVersion: operator.victoriametrics.com/v1beta1
    kind: VMAlert
    metadata:
      name: example-ha
      namespace: default
    spec:
      replicaCount: 2
      datasource:
        url: http://vmsingle-example.default.svc:8429
      notifiers:
        - url: http://vmalertmanager-example-0.vmalertmanager-example.default.svc:9093
        - url: http://vmalertmanager-example-1.vmalertmanager-example.default.svc:9093
      evaluationInterval: "10s"
      ruleSelector: {}
      # ...
    ```
- vmalert with service discovery:
    ```yaml
    apiVersion: operator.victoriametrics.com/v1beta1
    kind: VMAlert
    metadata:
      name: example-ha
      namespace: default
    spec:
      replicaCount: 2
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
      evaluationInterval: "10s"
      ruleSelector: {}
      # ...
    ```
  
In addition, you need to specify `remoteWrite` and `remoteRead` urls for restoring alert states after restarts:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-ha
  namespace: default
spec:
  replicaCount: 2
  evaluationInterval: "10s"
  selectAllByDefault: true
  datasource:
    url: http://vmselect-demo.vm.svc:8481/select/0/prometheus
  notifiers:
    - url: http://vmalertmanager-example-0.vmalertmanager-example.default.svc:9093
    - url: http://vmalertmanager-example-1.vmalertmanager-example.default.svc:9093
  remoteWrite:
    url: http://vminsert-demo.vm.svc:8480/insert/0/prometheus
  remoteRead:
    url: http://vmselect-demo.vm.svc:8481/select/0/prometheus
```

More details about `remoteWrite` and `remoteRead` you can read in [vmalert docs](https://docs.victoriametrics.com/vmalert.html#alerts-state-on-restarts).

## Version management

To set `VMAlert` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-vmalert
spec:
  image:
    repository: victoriametrics/vmalert
    tag: v1.93.4
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-vmalert
spec:
  image:
    repository: victoriametrics/vmalert
    tag: v1.93.4
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VMAlert` resource in the `spec` section of the `VMAlert` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: vmalert-resources-example
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
by default all `VMAlert` pods have resource requests and limits from the default values of the following [operator parameters](../configuration.md):

- `VM_VMALERTDEFAULT_RESOURCE_LIMIT_MEM` - default memory limit for `VMAlert` pods,
- `VM_VMALERTDEFAULT_RESOURCE_LIMIT_CPU` - default memory limit for `VMAlert` pods,
- `VM_VMALERTDEFAULT_RESOURCE_REQUEST_MEM` - default memory limit for `VMAlert` pods,
- `VM_VMALERTDEFAULT_RESOURCE_REQUEST_CPU` - default memory limit for `VMAlert` pods.

These default parameters will be used if:

- `VM_VMALERTDEFAULT_USEDEFAULTRESOURCES` is set to `true` (default value),
- `VMAlert` CR doesn't have `resources` field in `spec` section.

Field `resources` in `VMAlert` spec have higher priority than operator parameters.

If you set `VM_VMALERTDEFAULT_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VMAlert` CRD,
then `VMAlert` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Enterprise features

VMAlert supports features [Reading rules from object storage](https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage)
and [Multitenancy](https://docs.victoriametrics.com/vmalert.html#multitenancy)
from [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).

For using Enterprise version of [vmalert](https://docs.victoriametrics.com/vmalert.html)
you need to change version of `VMAlert` to version with `-enterprise` suffix using [Version management](#version-management).

All the enterprise apps require `-eula` command-line flag to be passed to them.
This flag acknowledges that your usage fits one of the cases listed on [this page](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).
So you can use [extraArgs](./README.md#extra-arguments) for passing this flag to `VMAlert`:

### Reading rules from object storage

After that you can pass `-rule` command-line argument with `s3://` or `gs://`
to `VMAlert` with [extraArgs](./README.md#extra-arguments).

More details about reading rules from object storage you can read in [vmalert docs](https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage).

Here are complete example for [Reading rules from object storage](https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: vmalert-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmalert
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmalert enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
    
    # using enterprise features: Reading rules from object storage
    # more details about reading rules from object storage you can read on https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage
    rule: s3://bucket/dir/alert.rules
    
  # ...other fields...
```

### Multitenancy

After enabling enterprise version you can use [Multitenancy](https://docs.victoriametrics.com/vmalert.html#multitenancy) 
feature in `VMAlert`.

For that you need to set `clusterMode` commad-line flag 
with [extraArgs](./README.md#extra-arguments) 
and specify `tenant` field for groups 
in [VMRule](./vmrule.md#enterprise-features):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: vmalert-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmalert
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmalert enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true

    # using enterprise features: Multitenancy
    # more details about multitenancy you can read on https://docs.victoriametrics.com/vmalert.html#multitenancy
    clusterMode: true 

  # ...other fields...

---

apiVersion: operator.victoriametrics.com/v1beta1
kind: VMRule
metadata:
  name: vmrule-ent-example
spec:
  groups:
    - name: vmalert-1
      rules:
        # using enterprise features: Multitenancy
        # more details about multitenancy you can read on https://docs.victoriametrics.com/vmalert.html#multitenancy
        - tenant: 1
          alert: vmalert config reload error
          expr: delta(vmalert_config_last_reload_errors_total[5m]) > 0
          for: 10s
          labels:
            severity: major
            job:  "{{ $labels.job }}"
          annotations:
            value: "{{ $value }}"
            description: 'error reloading vmalert config, reload count for 5 min {{ $value }}'
```

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlert
metadata:
  name: example-vmalert
spec:
  replicaCount: 1
  datasource:
    url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429"
  notifier:
    url: "http://vmalertmanager-example-alertmanager.default.svc:9093"
  evaluationInterval: "30s"
  selectAllByDefault: true
```
