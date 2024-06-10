---
sort: 3
weight: 3
title: VMAlertmanager
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 3
aliases:
  - /operator/resources/vmalertmanager.html
---

# VMAlertmanager

`VMAlertmanager` - represents [alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) configuration.

The `VMAlertmanager` CRD declaratively defines a desired Alertmanager setup to run in a Kubernetes cluster.
It provides options to configure replication and persistent storage.

For each `Alertmanager` resource, the Operator deploys a properly configured `StatefulSet` in the same namespace.
The Alertmanager pods are configured to include a `Secret` called `<alertmanager-name>` which holds the used
configuration file in the key `alertmanager.yaml`.

When there are two or more configured replicas the Operator runs the Alertmanager instances in high availability mode.

## Specification

You can see the full actual specification of the `VMAlertmanager` resource in the **[API docs -> VMAlertManager](../api.md#vmalertmanager)**.

If you can't find necessary field in the specification of the custom resource,
see [Extra arguments section](./README.md#extra-arguments).

Also, you can check out the [examples](#examples) section.

## Configuration

The operator generates a configuration file for `VMAlertmanager` based on user input at the definition of `CRD`.

Generated config stored at `Secret` created by the operator, it has the following name template `vmalertmanager-CRD_NAME-config`.

This configuration file is mounted at `VMAlertmanager` `Pod`. A special side-car container tracks its changes and sends config-reload signals to `alertmanager` container.

### Using secret

Basically, you can use the global configuration defined at manually created `Secret`. This `Secret` must be created before `VMAlertmanager`.

Name of the `Secret` must be defined at `VMAlertmanager` `spec.configSecret` option:

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
      receiver: 'webhook'
    receivers:
      - name: 'webhook'
        webhook_configs:
          - url: 'http://alertmanagerwh:30500/'

---

apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-alertmanager
spec:
  replicaCount: 2
  configSecret: vmalertmanager-example-alertmanager
```

### Using inline raw config

Also, if there is no secret data at configuration, or you just want to redefine some global variables for `alertmanager`.
You can define configuration at `spec.configRawYaml` section of `VMAlertmanager` configuration:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-alertmanager
spec:
  replicaCount: 2
  configRawYaml: |
    global:
       resolve_timeout: 5m
    route:
      receiver: 'default'
      group_interval: 5m
      repeat_interval: 12h
    receivers:
    - name: 'default'
```

If both `configSecret` and `configRawYaml` are defined, only configuration from `configRawYaml` will be used. Values from `configRawYaml` will be ignored.

### Using VMAlertmanagerConfig

See details at [VMAlertmanagerConfig](./vmalertmanagerconfig.md).

The CRD specifies which `VMAlertmanagerConfig`s should be covered by the deployed `VMAlertmanager` instances based on label selection.
The Operator then generates a configuration based on the included `VMAlertmanagerConfig`s and updates the `Configmaps` containing
the configuration. It continuously does so for all changes that are made to `VMAlertmanagerConfig`s or to the `VMAlertmanager` resource itself.

Configs are filtered by selectors `configNamespaceSelector` and `configSelector` in `VMAlertmanager` CRD definition.
For selecting rules from all namespaces you must specify it to empty value:

```yaml
spec:
  configSelector: {}
  configNamespaceSelector: {}
```

[VMAlertmanagerConfig](./vmalertmanagerconfig.md) objects are
generates part of [VMAlertmanager](./vmalertmanager.md) configuration.

For filtering rules `VMAlertmanager` uses selectors `configNamespaceSelector` and `configSelector`.
It allows configuring rules access control across namespaces and different environments.
Specification of selectors you can see in [this doc](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta).

In addition to the above selectors, the filtering of objects in a cluster is affected by the field `selectAllByDefault`
of `VMAlertmanager` spec and environment variable `WATCH_NAMESPACE` for operator.

Following rules are applied:

- If `configNamespaceSelector` and `configSelector` both undefined, then by default select nothing. With option set - `spec.selectAllByDefault: true`, select all vmalertmanagerconfigs.
- If `configNamespaceSelector` defined, `configSelector` undefined, then all vmalertmanagerconfigs are matching at namespaces for given `configNamespaceSelector`.
- If `configNamespaceSelector` undefined, `configSelector` defined, then all vmalertmanagerconfigs at `VMAlertmanager`'s namespaces are matching for given `configSelector`.
- If `configNamespaceSelector` and `configSelector` both defined, then only vmalertmanagerconfigs at namespaces matched `configNamespaceSelector` for given `configSelector` are matching.

Here's a more visual and more detailed view:

| `configNamespaceSelector` | `configSelector` | `selectAllByDefault` | `WATCH_NAMESPACE` | Selected rules                                                                                                         |
| ------------------------- | ---------------- | -------------------- | ----------------- | ---------------------------------------------------------------------------------------------------------------------- |
| undefined                 | undefined        | false                | undefined         | nothing                                                                                                                |
| undefined                 | undefined        | **true**             | undefined         | all vmalertmanagerconfigs in the cluster                                                                               |
| **defined**               | undefined        | *any*                | undefined         | all vmalertmanagerconfigs are matching at namespaces for given `configNamespaceSelector`                               |
| undefined                 | **defined**      | *any*                | undefined         | all vmalertmanagerconfigs only at `VMAlertmanager`'s namespace are matching for given `ruleSelector`                   |
| **defined**               | **defined**      | *any*                | undefined         | all vmalertmanagerconfigs only at namespaces matched `configNamespaceSelector` for given `configSelector` are matching |
| *any*                     | undefined        | *any*                | **defined**       | all vmalertmanagerconfigs only at `VMAlertmanager`'s namespace                                                         |
| *any*                     | **defined**      | *any*                | **defined**       | all vmalertmanagerconfigs only at `VMAlertmanager`'s namespace for given `configSelector` are matching                 |

More details about `WATCH_NAMESPACE` variable you can read in [this doc](../configuration.md#namespaced-mode).

Here are some examples of `VMAlertmanager` configuration with selectors:

```yaml
# select all config objects in the cluster
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: vmalertmanager-select-all
spec:
  # ...
  selectAllByDefault: true

---

# select all config objects in specific namespace (my-namespace)
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: vmalertmanager-select-ns
spec:
  # ...
  configNamespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
```

### Extra configuration files

`VMAlertmanager` specification has the following fields, that can be used to configure without editing raw configuration file:

- `spec.templates` - list of keys in `ConfigMaps`, that contains template files for `alertmanager`, e.g.:

  ```yaml
  apiVersion: operator.victoriametrics.com/v1beta1
  kind: VMAlertmanager
  metadata:
    name: example-alertmanager
  spec:
    replicaCount: 2
    templates:
      - name: alertmanager-templates
        key: my-template-1.tmpl
      - name: alertmanager-templates
        key: my-template-2.tmpl
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: alertmanager-templates
  data:
      my-template-1.tmpl: |
          {{ define "hello" -}}
          hello, Victoria!
          {{- end }}
      my-template-2.tmpl: """
  ```

These templates will be automatically added to `VMAlertmanager` configuration and will be automatically reloaded on changes in source `ConfigMap`.
- `spec.configMaps` - list of `ConfigMap` names (in the same namespace) that will be mounted at `VMAlertmanager`
  workload and will be automatically reloaded on changes in source `ConfigMap`. Mount path is `/etc/vm/configs/<configmap-name>`.

### Behavior without provided config

If no configuration is provided, operator configures stub configuration with blackhole route.

## High Availability

The final step of the high availability scheme is Alertmanager, when an alert triggers, actually fire alerts against *all* instances of an Alertmanager cluster.

The Alertmanager, starting with the `v0.5.0` release, ships with a high availability mode.
It implements a gossip protocol to synchronize instances of an Alertmanager cluster
regarding notifications that have been sent out, to prevent duplicate notifications.
It is an AP (available and partition tolerant) system. Being an AP system means that notifications are guaranteed to be sent at least once.

The Victoria Metrics Operator ensures that Alertmanager clusters are properly configured to run highly available on Kubernetes.

## Version management

To set `VMAlertmanager` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-vmalertmanager
spec:
  image:
    repository: prom/alertmanager
    tag: v0.25.0
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-vmalertmanager
spec:
  image:
    repository: prom/alertmanager
    tag: v0.25.0
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VMAlertManager` resource in the `spec` section of the `VMAlertManager` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertManager
metadata:
  name: vmalertmanager-resources-example
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
by default all `VMAlertManager` pods have resource requests and limits from the default values of the following [operator parameters](../configuration.md):

- `VM_VMALERTMANAGER_RESOURCE_LIMIT_MEM` - default memory limit for `VMAlertManager` pods,
- `VM_VMALERTMANAGER_RESOURCE_LIMIT_CPU` - default memory limit for `VMAlertManager` pods,
- `VM_VMALERTMANAGER_RESOURCE_REQUEST_MEM` - default memory limit for `VMAlertManager` pods,
- `VM_VMALERTMANAGER_RESOURCE_REQUEST_CPU` - default memory limit for `VMAlertManager` pods.

These default parameters will be used if:

- `VM_VMALERTMANAGER_USEDEFAULTRESOURCES` is set to `true` (default value),
- `VMAlertManager` CR doesn't have `resources` field in `spec` section.

Field `resources` in `VMAlertManager` spec have higher priority than operator parameters.

If you set `VM_VMALERTMANAGER_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VMAlertManager` CRD,
then `VMAlertManager` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: vmalertmanager-example
spec:
  replicaCount: 1
  configRawYaml: |
        global:
          resolve_timeout: 5m
        route:
          group_wait: 30s
          group_interval: 5m
          repeat_interval: 12h
          receiver: 'webhook'
        receivers:
        - name: 'webhook'
          webhook_configs:
          - url: 'http://localhost:30502/'
```
