---
sort: 7
weight: 7
title: Authorization and exposing components
menu:
  docs:
    parent: "operator"
    weight: 7
aliases:
  - /operator/auth.html
---

# Authorization and exposing components

## Exposing components

CRD objects doesn't have `ingress` configuration. 
Instead, you can use [VMAuth](./resources/vmauth.md) as proxy between ingress-controller and VictoriaMetrics components.

It adds missing authorization and access control features and enforces it.

Access can be given with [VMUser](./resources/vmuser.md) definition. 

It supports basic auth and bearer token authentication:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: main-router
spec:
  userNamespaceSelector: {}
  userSelector: {}
  ingress: {}
  unauthorizedAccessConfig: []
```

Advanced configuration with cert-manager annotations:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: router-main
spec:
  podMetadata:
    labels:
      component: vmauth
  userSelector: {}
  userNamespaceSelector: {}
  replicaCount: 2
  resources:
    requests:
      cpu: "250m"
      memory: "350Mi"
    limits:
      cpu: "500m"
      memory: "850Mi"
  ingress:
    tlsSecretName: vmauth-tls
    annotations:
      cert-manager.io/cluster-issuer: base
    class_name: nginx
    tlsHosts:
      - vm-access.example.com
```

Simple static routing with read-only access to vmagent for username - `user-1` with password `Asafs124142`:

```yaml
# curl vmauth:8427/metrics -u 'user-1:Asafs124142'
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
  name: user-1
spec:
  password: Asafs124142
  targetRefs:
    - static:
        url: http://vmagent-base.default.svc:8429
      paths: ["/targets/api/v1","/targets","/metrics"]
```

With bearer token access:

```yaml
# curl vmauth:8427/metrics -H 'Authorization: Bearer Asafs124142'
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
  name: user-2
spec:
  bearerToken: Asafs124142
  targetRefs:
    - static:
        url: http://vmagent-base.default.svc:8429
      paths: ["/targets/api/v1","/targets","/metrics"]
```

It's also possible to use service discovery for objects:

```yaml
# curl vmauth:8427/metrics -H 'Authorization: Bearer Asafs124142'
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
  name: user-3
spec:
  bearerToken: Asafs124142
  targetRefs:
    - crd:
        kind: VMAgent
        name: base
        namespace: default
      paths: ["/targets/api/v1","/targets","/metrics"]
```

Cluster components supports auto path generation for single tenant view:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
 name: vmuser-tenant-1
spec:
 bearerToken: some-token
 targetRefs:
  - crd:
     kind: VMCluster/vminsert
     name: test-persistent
     namespace: default
    target_path_suffix: "/insert/1"
  - crd:
     kind: VMCluster/vmselect
     name: test-persistent
     namespace: default
    target_path_suffix: "/select/1"
  - static:
     url: http://vmselect-test-persistent.default.svc:8481/
    paths:
     - /internal/resetRollupResultCache
```

For each `VMUser` operator generates corresponding secret with username/password or bearer token at the same namespace as `VMUser`.

## Basic auth for targets

To authenticate a `VMServiceScrape`s over a metrics endpoint use [`basicAuth`](./api.md#basicauth):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  labels:
    k8s-apps: basic-auth-example
  name: basic-auth-example
spec:
  endpoints:
  - basicAuth:
      password:
        name: basic-auth
        key: password
      username:
        name: basic-auth
        key: user
    port: metrics
  selector:
    matchLabels:
      app: myapp

---

apiVersion: v1
kind: Secret
metadata:
  name: basic-auth
data:
  password: dG9vcg== # toor
  user: YWRtaW4= # admin
type: Opaque
```

## Unauthorized access

You can expose some routes without authorization with `unauthorizedAccessConfig`.

Check more details in [VMAuth docs -> Unauthorized access](./resources/vmauth.md#unauthorized-access).

More details about features of `VMAuth` and `VMUser` you can read in:
- [VMAuth docs](./resources/vmauth.md),
- [VMUser docs](./resources/vmuser.md).
