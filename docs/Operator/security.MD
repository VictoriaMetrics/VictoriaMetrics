---
sort: 12
---

# Security

VictoriaMetrics operator provides several security features, such as [PodSecurityPolicies](https://kubernetes.io/docs/concepts/policy/pod-security-policy/), [PodSecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).


## PodSecurityPolicy.

 By default, operator creates serviceAccount for each cluster resource and binds default `PodSecurityPolicy` to it.

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

 This behaviour may be disabled with env variable passed to operator:
 ```yaml
 - name: VM_PSPAUTOCREATEENABLED
   value: "false"
```

 User may also override default pod security policy with setting: `spec.podSecurityPolicyName: "psp-name"`.
 

## PodSecurityContext

 `PodSecurityContext` can be configured with spec setting. It may be useful for mounted volumes, with `VMSingle` for example:
 
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
