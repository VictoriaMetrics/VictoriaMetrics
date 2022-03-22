---
sort: 4
---

# Authorization and exposing components

## Exposing components


  CRD objects doesn't have `ingress` configuration. Instead, you can use `VMAuth` as proxy between ingress-controller and VM app components.
 It adds missing authorization and access control features and enforces it.

 Access can be given with `VMUser` definition. It supports  basic auth and bearer token authentication.

```yaml
cat << EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAuth
metadata:
  name: main-router
spec:
  userNamespaceSelector: {}
  userSelector: {}
  ingress: {}
EOF
```

 Advanced configuration with cert-manager annotations:
```yaml
cat << EOF | kubectl apply -f -
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
EOF
```
 

simple static routing with read-only access to vmagent for username - `user-1` with password `Asafs124142`
```yaml
# curl vmauth:8427/metrics -u 'user-1:Asafs124142'
cat  << EOF | kubectl apply -f
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
EOF
```

  With bearer token access:

```yaml
# curl vmauth:8427/metrics -H 'Authorization: Bearer Asafs124142'
cat  << EOF | kubectl apply -f
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
EOF
```

 It's also possible to use service discovery for objects:
```yaml
# curl vmauth:8427/metrics -H 'Authorization: Bearer Asafs124142'
cat  << EOF | kubectl apply -f
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
EOF
```

 Cluster components supports auto path generation for single tenant view:
```yaml
cat << EOF | kubectl apply -f -
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
EOF
```

 For each `VMUser` operator generates corresponding secret with username/password or bearer token at the same namespace as `VMUser`.

## Basic auth for targets

To authenticate a `VMServiceScrape`s over a metrics endpoint use [`basicAuth`](https://docs.victoriametrics.com/operator/api.html#basicauth)

```yaml
cat <<EOF | kubectl apply -f -
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
EOF
```

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: basic-auth
data:
  password: dG9vcg== # toor
  user: YWRtaW4= # admin
type: Opaque
EOF
```
