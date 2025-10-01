---
title: OpenShift
weight: 10
menu:
  docs:
    parent: "integrations-vm"
    weight: 10
---

## OpenShift Container Platform (OCP)

OpenShift uses Prometheus as a core monitoring solution. It cannot be replaced without violating the OpenShift support policy. However, OpenShift can be configured to use VictoriaMetrics as a remote write target.

According to [remote write configuration in the OpenShift documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/monitoring/configuring-core-platform-monitoring#configuring-remote-write-storage_configuring-metrics) the following manifest needs to be applied to send platform metrics to VictoriaMetrics:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    prometheusK8s:
      remoteWrite:
      - url: "https://<victoriametrics_url>/api/v1/write"
```
This instructs Prometheus to push metrics to VictoriaMetrics instance via Prometheus remote write protocol. This URL may also point to vminsert and contain [tenant ID](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    prometheusK8s:
      remoteWrite:
      - url: "https://<vminsert-addr>/insert/<tenant_id>/prometheus/api/v1/write"
```

Note, that OpenShift uses two Prometheus replicas for HA configuration. Each replica adds `prometheus_replica` label to the forwarded metrics. This is excessive on VictoriaMetrics side, so use the following setting to drop this label:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    prometheusK8s:
      remoteWrite:
      - url: "..."
        writeRelabelConfigs:
        - regex: 'prometheus_replica'
          action: labeldrop
```

We also recommend setting Prometheus retention to the minimum:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    prometheusK8s:
      retention: "1h"
```

In case VictoriaMetrics requires authentication, you can configure it by adding the following lines to the `config.yaml` file:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    prometheusK8s:
      remoteWrite:
      - url: "..."
        authorization:
          type: Bearer
          credentials:
            name: config-map-bearer
            key: token
```

Along with core platform monitoring, OpenShift also supports collecting user workload metrics. See [this guide](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/monitoring/configuring-user-workload-monitoring) for more information. In order to send user workload metrics to VictoriaMetrics, the following manifest needs to be applied:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: user-workload-monitoring-config
  namespace: openshift-user-workload-monitoring
data:
  config.yaml: |
    prometheus:
      remoteWrite:
      - url: "https://<vminsert-addr>/insert/<tenant_id>/prometheus/api/v1/write"
        writeRelabelConfigs:
        - regex: 'prometheus_replica'
          action: labeldrop
```

## References

- [OpenShift Documentation: Monitoring](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/monitoring/configuring-core-platform-monitoring)
