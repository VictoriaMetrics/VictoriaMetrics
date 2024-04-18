---
sort: 15
weight: 15
title: VMUser
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 15
aliases:
  - /operator/resources/vmuser.html
---

# VMUser

The `VMUser` CRD describes user configuration, its authentication methods `basic auth` or `Authorization` header. 
User access permissions, with possible routing information.

User can define routing target with `static` config, by entering target `url`, or with `CRDRef`, in this case, 
operator queries kubernetes API, retrieves information about CRD and builds proper url.

## Specification

You can see the full actual specification of the `VMUser` resource in
the **[API docs -> VMUser](../api.md#vmuser)**.

Also, you can check out the [examples](#examples) section.

## Authentication methods

There are two authentication mechanisms: ["Bearer token"](#bearer-token) and ["Basic auth"](#basic-auth) with `username` and `password`. 
Only one of them can be used with `VMUser` at one time.

Operator creates `Secret` for every `VMUser` with name - `vmuser-{VMUser.metadata.name}`.
It places `username` + `password` or `bearerToken` into `data` section.

### Bearer token

Bearer token is a way to authenticate user with `Authorization` header. 
User defines `token` field in `auth` section.

Also, you can check out the [examples](#examples) section.

### Basic auth

Basic auth is the simplest way to authenticate user. User defines `username` and `password` fields in `auth` section.

If `username` is empty, `metadata.name` from `VMUser` used as `username`.

You can automatically generate `password` if:
- Set `generatePassword: true` field
- Don't fill `password` field

Operator generates random password for this `VMUser`, 
this password will be added to the `Secret` for this `VMUser` at `data.password` field.

Also, you can check out the [examples](#examples) section.

## Routing

You can define routes for user in `targetRefs` section. 

For every entry in `targetRefs` you can define routing target with `static` config, by entering target `url`, 
or with `crd`, in this case, operator queries kubernetes API, retrieves information about CRD and builds proper url.

Here are details about other fields in `targetRefs`:

- `paths` is the same as `src_paths` from [auth config](https://docs.victoriametrics.com/vmauth.html#auth-config)
- `headers` is the same as `headers` from [auth config](https://docs.victoriametrics.com/vmauth.html#auth-config)
- `targetPathSuffix` is the suffix for `url_prefix` (target URL) from [auth config](https://docs.victoriametrics.com/vmauth.html#auth-config)

### Static

The `static` field is the same as `url_prefix` (target URL) from [auth config](https://docs.victoriametrics.com/vmauth.html#auth-config),
it allows you to set a specific static URL.

### CRDRef

The `crd` field is a more convenient form for specifying the components handled by the operator as auth targets.

User can define routing target with `crd` config, by entering `kind`, `name` and `namespace` of CRD.

Operator supports following kinds in `kind` field:

- `VMAgent` for [VMAgent](./vmagent.md)
- `VMAlert` for [VMAlert](./vmalert.md)
- `VMAlertmanager` for [VMAlertmanager](./vmalertmanager.md)
- `VMSingle` for [VMSingle](./vmsingle.md)
- `VMCluster/vmselect`, `VMCluster/vminsert` and `VMCluster/vmstorage` for [VMCluster](./vmcluster.md)

Also, you can check out the [examples](#examples) section.

Additional fields like `path` and `scheme` can be added to `CRDRef` config.

## Enterprise features

Custom resource `VMUser` supports feature [IP filters](https://docs.victoriametrics.com/vmauth.html#ip-filters) 
from [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).

### IP Filters

For using [IP filters](https://docs.victoriametrics.com/vmauth.html#ip-filters) 
you need to **[enable VMAuth Enterprise](./vmauth.md#enterprise-features)**.

After that you can add `ip_filters` field to `VMUser`:

```yaml
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
kind: VMUser
metadata:
  name: example
spec:
  username: simple-user
  password: simple-password
  targetRefs:
    - crd:
        kind: VMSingle
        name: example
        namespace: default
      paths: ["/.*"]
    - static:
        url: http://vmalert-example.default.svc:8080
      paths: ["/api/v1/groups","/api/v1/alerts"]
```

More examples see on [Authorization and exposing components](../auth.md) page
and in [Quickstart guide](../quick-start.md#vmuser).
