---
sort: 3
weight: 3
title: Security
menu:
  docs:
    parent: "operator"
    weight: 3
aliases:
  - /operator/security.html
---

# Security

## Access control

### Roles

To run in a cluster the operator needs certain permissions, you can see them in [this directory](https://github.com/VictoriaMetrics/operator/tree/master/config/rbac):

- [`role.yaml` file](https://github.com/VictoriaMetrics/operator/blob/master/config/rbac/role.yaml) - basic set of cluster roles for launching an operator.
- [`leader_election_role.yaml` file](https://github.com/VictoriaMetrics/operator/blob/master/config/rbac/leader_election_role.yaml) - set of roles with permissions to do leader election (is necessary to run the operator in several replicas for high availability).

Also, you can use single-namespace mode with minimal permissions, see [this section](./configuration.md#namespaced-mode) for details.

Also in [the same directory](https://github.com/VictoriaMetrics/operator/tree/master/config/rbac) are files with a set of separate permissions to view or edit [operator resources](./resources/README.md) to organize fine-grained access:

- file `<RESOURCE_NAME>_viewer_role.yaml` - permissions for viewing (`get`, `list` and `watch`) some resource of vmoperator.
- file `<RESOURCE_NAME>_editor_role.yaml` - permissions for editing (`create`, `delete`, `patch`, `update` and `deletecollection`) some resource of vmoperator (also includes viewing permissions).

For instance, [`vmalert_editor_role.yaml` file](https://github.com/VictoriaMetrics/operator/blob/master/config/rbac/vmalert_editor_role.yaml) contain permission
for editing [`vmagent` custom resources](./resources/vmagent.md).

<!-- TODO: service accounts / role bindings? -->
<!-- TODO: resource/roles relations -->

## Security policies

VictoriaMetrics operator provides several security features, such as [PodSecurityPolicies](https://kubernetes.io/docs/concepts/policy/pod-security-policy/),
[PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).

### PodSecurityPolicy

> PodSecurityPolicy was [deprecated](https://kubernetes.io/docs/concepts/security/pod-security-policy/) in Kubernetes v1.21, and removed from Kubernetes in v1.25.

If your Kubernetes version is under v1.25 and want to use PodSecurityPolicy, you can set env `VM_PSPAUTOCREATEENABLED: "true"` in operator, it will create serviceAccount for each cluster resource and binds default `PodSecurityPolicy` to it.

Default psp:
```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: vmagent-example-vmagent
spec:
  allowPrivilegeEscalation: false
  fsGroup:
    rule: RunAsAny
  hostNetwork: true
  requiredDropCapabilities:
  - ALL
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - persistentVolumeClaim
  - secret
  - emptyDir
  - configMap
  - projected
  - downwardAPI
  - nfs
```

User may also override default pod security policy with setting: `spec.podSecurityPolicyName: "psp-name"`.

## PodSecurityContext

VictoriaMetrics operator will add default Security Context to managed pods and containers if env `EnableStrictSecurity: "true"` is set.
The following SecurityContext will be applied:

### Pod SecurityContext

1. **RunAsNonRoot: true**
1. **RunAsUser/RunAsGroup/FSGroup: 65534**

    '65534' refers to 'nobody' in all the used default images like alpine, busybox.

    If you're using customize image, please make sure '65534' is a valid uid in there or specify SecurityContext.
1. **FSGroupChangePolicy: &onRootMismatch**
  
    If KubeVersion>=1.20, use `FSGroupChangePolicy="onRootMismatch"` to skip the recursive permission change
    when the root of the volume already has the correct permissions
1. **SeccompProfile: {type: RuntimeDefault}**

    Use `RuntimeDefault` seccomp profile by default, which is defined by the container runtime,
    instead of using the Unconfined (seccomp disabled) mode.

### Container SecurityContext

1. **AllowPrivilegeEscalation: false**
1. **ReadOnlyRootFilesystem: true**
1. **Capabilities: {drop: [all]}**


Also `SecurityContext` can be configured with spec setting. It may be useful for mounted volumes, with `VMSingle` for example:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: vmsingle-f
  namespace: monitoring-system
spec:
  retentionPeriod: "2"
  removePvcAfterDelete: true
  securityContext:
      runAsUser: 1000
      fsGroup: 1000
      runAsGroup: 1000
  extraArgs:
    dedup.minScrapeInterval: 10s
  storage:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 25Gi
  resources:
    requests:
      cpu: "0.5"
      memory: "512Mi"
    limits:
      cpu: "1"
      memory: "1512Mi"
```
