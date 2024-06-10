---
sort: 9
weight: 9
title: VMProbe
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 9
aliases:
  - /operator/resources/vmprobe.html
---

# VMProbe

The `VMProbe` CRD provides probing target ability with some external prober. 
The most common prober is [blackbox exporter](https://github.com/prometheus/blackbox_exporter).
By specifying configuration at CRD, operator generates config for [VMAgent](./vmagent.md)
and syncs it. It's possible to use static targets or use standard k8s discovery mechanism with `Ingress`.

`VMProbe` object generates part of [VMAgent](./vmagent.md) configuration;
It has various options for scraping configuration of target (with basic auth, tls access, by specific port name etc.).

You have to configure blackbox exporter before you can use this feature. 
The second requirement is [VMAgent](./vmagent.md) selectors,
it must match your `VMProbe` by label or namespace selector. `VMAgent` `probeSelector` must match `VMProbe` labels.

See more details about selectors [here](./vmagent.md#scraping).

## Specification

You can see the full actual specification of the `VMProbe` resource in
the **[API docs -> VMProbe](../api.md#vmprobe)**.

Also, you can check out the [examples](#examples) section.

## Migration from Prometheus

The `VMProbe` CRD from VictoriaMetrics Operator is a drop-in replacement
for the Prometheus `Probe` from prometheus-operator.

More details about migration from prometheus-operator you can read in [this doc](../migration.md).

## Examples

### Static targets

It will probe `VMAgent` with url - `vmagent-example-vmagent.default.svc:9115/heath` with blackbox url:
`prometheus-blackbox-exporter.default.svc:9115` and module `http_2xx` 
(it was specified at [blackbox configmap](#blackbox-exporter)).

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMProbe
metadata:
  name: vmprobe-static-example
spec:
  jobName: static-probe
  vmProberSpec:
     # by default scheme http, and path is /probe
     url: prometheus-blackbox-exporter.default.svc:9115
  module: http_2xx
  targets:
   staticConfig: 
      targets:
      -  vmagent-example-vmagent.default.svc:8429/health
  interval: 2s 
```

After adding target to `VMAgent` configuration it starts probing itself throw blackbox exporter.

### Ingress targets

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMProbe
metadata:
  name: vmprobe-ingress-example
spec:
  vmProberSpec:
     # by default scheme http, and path is /probe
     url: prometheus-blackbox-exporter.default.svc:9115
  module: http_2xx
  targets:
   ingress: 
      selector:
       matchLabels:
        app: victoria-metrics-single
  interval: 10s
```

This configuration will add 2 additional targets for probing: `vmsingle2.example.com` and `vmsingle.example.com`.

But probes will be unsuccessful, because there is no such hosts.

### Related resources

Following resources will be used for the examples below:

#### Blackbox exporter

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-blackbox-exporter
  labels:
    app: prometheus-blackbox-exporter
data:
  blackbox.yaml: |
    modules:
      http_2xx:
        http:
          preferred_ip_protocol: ip4
          valid_http_versions:
          - HTTP/1.1
          - HTTP/2.0
          valid_status_codes: []
        prober: http
        timeout: 5s

---

kind: Service
apiVersion: v1
metadata:
  name: prometheus-blackbox-exporter
  labels:
    app: prometheus-blackbox-exporter
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 9115
      protocol: TCP
  selector:
    app: prometheus-blackbox-exporter

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus-blackbox-exporter
  labels:
    app: prometheus-blackbox-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus-blackbox-exporter
  template:
    metadata:
      labels:
        app: prometheus-blackbox-exporter
    spec:
      containers:
        - name: blackbox-exporter
          image: "prom/blackbox-exporter:v0.17.0"
          args:
            - "--config.file=/config/blackbox.yaml"
          resources:
            {}
          ports:
            - containerPort: 9115
              name: http
          livenessProbe:
            httpGet:
              path: /health
              port: http
          readinessProbe:
            httpGet:
              path: /health
              port: http
          volumeMounts:
            - mountPath: /config
              name: config
      volumes:
        - name: config
          configMap:
            name: prometheus-blackbox-exporter
```

### VMSingle

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle-persisted
spec:
  retentionPeriod: "1"
  removePvcAfterDelete: true
  storage:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi 

---

apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  labels:
    app: victoria-metrics-single
  name: victoria-metrics-single
spec:
  rules:
    - host: vmsingle.example.com
      http:
        paths:
          - backend:
              serviceName: vmsingle-example-vmsingle-persisted
              servicePort: 8428
            path: /
    - host: vmsingle2.example.com
      http:
        paths:
          - backend:
              serviceName: vmsingle-example-vmsingle-persisted
              servicePort: 8428
            path: /
```

### VMAgent

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
   name: example-vmagent
spec:
   selectAllByDefault: true
   replicaCount: 1
   remoteWrite:
     - url: "http://vmsingle-example-vmsingle-persisted.default.svc:8429/api/v1/write"
```
