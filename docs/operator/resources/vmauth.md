---
sort: 5
weight: 5
title: VMAuth
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 5
aliases:
  - /operator/resources/vmauth.html
---

# VMAuth

The `VMAuth` CRD provides mechanism for exposing application with authorization to outside world or to other applications inside kubernetes cluster.

For first case, user can configure `ingress` setting at `VMAuth` CRD. For second one, operator will create secret with `username` and `password` at `VMUser` CRD name.
So it will be possible to access these credentials from any application by targeting corresponding kubernetes secret.

## Specification

You can see the full actual specification of the `VMAuth` resource in
the **[API docs -> VMAuth](../api.md#vmauth)**.

If you can't find necessary field in the specification of the custom resource,
see [Extra arguments section](./README.md#extra-arguments).

Also, you can check out the [examples](#examples) section.

## Users

The CRD specifies which `VMUser`s should be covered by the deployed `VMAuth` instances based on label selection.
The Operator then generates a configuration based on the included `VMUser`s and updates the `Configmaps` containing
the configuration. It continuously does so for all changes that are made to `VMUser`s or to the `VMAuth` resource itself.

[VMUser](./vmuser.md) objects generate part of `VMAuth` configuration.

For filtering users `VMAuth` uses selectors `userNamespaceSelector` and `userSelector`.
It allows configuring rules access control across namespaces and different environments.
Specification of selectors you can see in [this doc](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#labelselector-v1-meta).

In addition to the above selectors, the filtering of objects in a cluster is affected by the field `selectAllByDefault` of `VMAuth` spec and environment variable `WATCH_NAMESPACE` for operator.

Following rules are applied:

- If `userNamespaceSelector` and `userSelector` both undefined, then by default select nothing. With option set - `spec.selectAllByDefault: true`, select all vmusers.
- If `userNamespaceSelector` defined, `userSelector` undefined, then all vmusers are matching at namespaces for given `userNamespaceSelector`.
- If `userNamespaceSelector` undefined, `userSelector` defined, then all vmusers at `VMAgent`'s namespaces are matching for given `userSelector`.
- If `userNamespaceSelector` and `userSelector` both defined, then only vmusers at namespaces matched `userNamespaceSelector` for given `userSelector` are matching.

Here's a more visual and more detailed view:

| `userNamespaceSelector` | `userSelector` | `selectAllByDefault` | `WATCH_NAMESPACE` | Selected rules                                                                                       |
|-------------------------|----------------|----------------------|-------------------|------------------------------------------------------------------------------------------------------|
| undefined               | undefined      | false                | undefined         | nothing                                                                                              |
| undefined               | undefined      | **true**             | undefined         | all vmusers in the cluster                                                                           |
| **defined**             | undefined      | *any*                | undefined         | all vmusers are matching at namespaces for given `userNamespaceSelector`                             |
| undefined               | **defined**    | *any*                | undefined         | all vmusers only at `VMAuth`'s namespace are matching for given `userSelector`                       |
| **defined**             | **defined**    | *any*                | undefined         | all vmusers only at namespaces matched `userNamespaceSelector` for given `userSelector` are matching |
| *any*                   | undefined      | *any*                | **defined**       | all vmusers only at `VMAuth`'s namespace                                                             |
| *any*                   | **defined**    | *any*                | **defined**       | all vmusers only at `VMAuth`'s namespace for given `userSelector` are matching                       |

More details about `WATCH_NAMESPACE` variable you can read in [this doc](../configuration.md#namespaced-mode).

Here are some examples of `VMAuth` configuration with selectors:

```yaml
# select all user objects in the cluster
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-select-all
spec:
  # ...
  selectAllByDefault: true

---

# select all user objects in specific namespace (my-namespace)
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-select-ns
spec:
  # ...
  userNamespaceSelector: 
    matchLabels:
      kubernetes.io/metadata.name: my-namespace
```

## Unauthorized access

You can configure `VMAuth` to allow unauthorized access for specified routes with `unauthorizedAccessConfig` field.

For instance:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-unauthorized-example
spec:
  unauthorizedAccessConfig:
    - src_paths: ["/metrics"]
      url_prefix:
        - http://vmsingle-example.default.svc:8428
```

In this example every user can access `/metrics` route and get vmsingle metrics without authorization.

In addition, `unauthorizedAccessConfig` in [Enterprise version](#enterprise-features) supports [IP Filters](#ip-filters) 
with `ip_filters` field.

## High availability

The `VMAuth` resource is stateless, so it can be scaled horizontally by increasing the number of replicas:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-example
spec:
    replicas: 3
    # ...
```

## Version management

To set `VMAuth` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: example-vmauth
spec:
  image:
    repository: victoriametrics/vmauth
    tag: v1.93.4
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: example-vmauth
spec:
  image:
    repository: victoriametrics/vmauth
    tag: v1.93.4
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VMAuth` resource in the `spec` section of the `VMAuth` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-resources-example
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
by default all `VMAuth` pods have resource requests and limits from the default values of the following [operator parameters](../configuration.md):

- `VM_VMAUTHDEFAULT_RESOURCE_LIMIT_MEM` - default memory limit for `VMAuth` pods,
- `VM_VMAUTHDEFAULT_RESOURCE_LIMIT_CPU` - default memory limit for `VMAuth` pods,
- `VM_VMAUTHDEFAULT_RESOURCE_REQUEST_MEM` - default memory limit for `VMAuth` pods,
- `VM_VMAUTHDEFAULT_RESOURCE_REQUEST_CPU` - default memory limit for `VMAuth` pods.

These default parameters will be used if:

- `VM_VMAUTHDEFAULT_USEDEFAULTRESOURCES` is set to `true` (default value),
- `VMAuth` CR doesn't have `resources` field in `spec` section.

Field `resources` in `VMAuth` spec have higher priority than operator parameters.

If you set `VM_VMAUTHDEFAULT_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VMAuth` CRD,
then `VMAuth` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Enterprise features

Custom resource `VMAuth` supports feature [IP filters](https://docs.victoriametrics.com/vmauth.html#ip-filters)
from [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).

For using Enterprise version of [vmauth](https://docs.victoriametrics.com/vmauth.html) 
you need to change version of `vmauth` to version with `-enterprise` suffix using [Version management](#version-management).

All the enterprise apps require `-eula` command-line flag to be passed to them. 
This flag acknowledges that your usage fits one of the cases listed on [this page](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).
So you can use [extraArgs](./README.md#extra-arguments) for passing this flag to `VMAuth`:

### IP Filters

After that you can use [IP filters for `VMUser`](./vmuser.md#enterprise-features) 
and field `ip_filters` for `VMAuth`.

Here are complete example with described above:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: vmauth-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmauth
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmauth enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
  
  # using enterprise features: ip filters for vmauth
  # more details about ip filters you can read in https://docs.victoriametrics.com/vmauth.html#ip-filters
  ip_filters:
    allow_list:
      - 10.0.0.0/24
      - 1.2.3.4
    deny_list:
      - 5.6.7.8
  # allow read vmsingle metrics without authorization for users from internal network
  unauthorizedAccessConfig:
    - src_paths: ["/metrics"]
      url_prefix: ["http://vmsingle-example.default.svc:8428"]
      ip_filters:
        allow_list:
          - 192.168.0.0/16
          - 10.0.0.0/8

  # ...other fields...

---

apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
  name: vmuser-ent-example
spec:
  username: simple-user
  password: simple-password

  # using enterprise features: ip filters for vmuser
  # more details about ip filters you can read in https://docs.victoriametrics.com/vmuser.html#enterprise-features
  ip_filters:
    allow_list:
      - 10.0.0.0/24
      - 1.2.3.4
    deny_list:
      - 5.6.7.8
```

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: example
  namespace: default
spec:
  selectAllByDefault: true
  ingress:
    class_name: nginx # <-- change this to your ingress-controller
    host: vm-demo.k8s.orb.local # <-- change this to your domain
```
