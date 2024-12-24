---
weight: 20
title: VLogs
menu:
  docs:
    identifier: operator-cr-vlogs
    parent: operator-cr
    weight: 20
aliases:
  - /operator/resources/vlogs/
  - /operator/resources/vlogs/index.html
---
`VLogs` represents database for storing logs.
The `VLogs` CRD declaratively defines a [single-node VictoriaLogs](https://docs.victoriametrics.com/victorialogs/)
installation to run in a Kubernetes cluster.

For each `VLogs` resource, the Operator deploys a properly configured `Deployment` in the same namespace.
The VLogs `Pod`s are configured to mount an empty dir or `PersistentVolumeClaimSpec` for storing data.
Deployment update strategy set to [recreate](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#recreate-deployment).
No more than one replica allowed.

For each `VLogs` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with name from `VLogs.metadata.name`.

## Specification

You can see the full actual specification of the `VLogs` resource in the **[API docs -> VLogs](https://docs.victoriametrics.com/operator/api#vlogs)**.

If you can't find necessary field in the specification of the custom resource,
see [Extra arguments section](./#extra-arguments).

Also, you can check out the [examples](#examples) section.

## High availability

`VLogs` doesn't support high availability. Consider using [`victorialogs-single chart`](https://docs.victoriametrics.com/helm/victorialogs-single/), where it's possible to configure relica count in statefulset mode for such purpose.

## Version management

To set `VLogs` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VLogs
metadata:
  name: example-vlogs
spec:
  image:
    repository: victoriametrics/victoria-logs
    tag: v1.4.0
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VLogs
metadata:
  name: example-vlogs
spec:
  image:
    repository: victoriametrics/victoria-logs
    tag: v1.4.0
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VLogs` resource in the `spec` section of the `VLogs` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VLogs
metadata:
  name: resources-example
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
by default all `VLogs` pods have resource requests and limits from the default values of the following [operator parameters](https://docs.victoriametrics.com/operator/configuration):

- `VM_VLOGSDEFAULT_RESOURCE_LIMIT_MEM` - default memory limit for `VLogs` pods,
- `VM_VLOGSDEFAULT_RESOURCE_LIMIT_CPU` - default memory limit for `VLogs` pods,
- `VM_VLOGSDEFAULT_RESOURCE_REQUEST_MEM` - default memory limit for `VLogs` pods,
- `VM_VLOGSDEFAULT_RESOURCE_REQUEST_CPU` - default memory limit for `VLogs` pods.

These default parameters will be used if:

- `VM_VLOGSDEFAULT_USEDEFAULTRESOURCES` is set to `true` (default value),
- `VLogs` CR doesn't have `resources` field in `spec` section.

Field `resources` in `VLogs` spec have higher priority than operator parameters.

If you set `VM_VLOGSDEFAULT_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VLogs` CRD,
then `VLogs` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Examples

```yaml
kind: VLogs
metadata:
  name: example
spec:
  retentionPeriod: "12"
  removePvcAfterDelete: true
  storage:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 50Gi
  resources:
    requests:
      memory: 500Mi
      cpu: 500m
    limits:
      memory: 10Gi
      cpu: 5
```
