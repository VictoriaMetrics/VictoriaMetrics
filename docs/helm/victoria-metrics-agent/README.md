![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.12.2](https://img.shields.io/badge/Version-0.12.2-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-agent)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Metrics Agent - collects metrics from various sources and stores them to VictoriaMetrics

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](../../REQUIREMENTS.md).

## How to install

Access a Kubernetes cluster.

### Setup chart repository (can be omitted for OCI repositories)

Add a chart helm repository with follow commands:

```console
helm repo add vm https://victoriametrics.github.io/helm-charts/

helm repo update
```
List versions of `vm/victoria-metrics-agent` chart available to installation:

```console
helm search repo vm/victoria-metrics-agent -l
```

### Install `victoria-metrics-agent` chart

Export default values of `victoria-metrics-agent` chart to file `values.yaml`:

  - For HTTPS repository

    ```console
    helm show values vm/victoria-metrics-agent > values.yaml
    ```
  - For OCI repository

    ```console
    helm show values oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-agent > values.yaml
    ```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

  - For HTTPS repository

    ```console
    helm install vma vm/victoria-metrics-agent -f values.yaml -n NAMESPACE --debug --dry-run
    ```

  - For OCI repository

    ```console
    helm install vma oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-agent -f values.yaml -n NAMESPACE --debug --dry-run
    ```

Install chart with command:

  - For HTTPS repository

    ```console
    helm install vma vm/victoria-metrics-agent -f values.yaml -n NAMESPACE
    ```

  - For OCI repository

    ```console
    helm install vma oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-agent -f values.yaml -n NAMESPACE
    ```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'vma'
```

Get the application by running this command:

```console
helm list -f vma -n NAMESPACE
```

See the history of versions of `vma` application with command.

```console
helm history vma -n NAMESPACE
```

## How to uninstall

Remove application with command.

```console
helm uninstall vma -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](../../REQUIREMENTS.md).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-agent

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-agent/values.yaml`` file.

<table>
  <thead>
    <th>Key</th>
    <th>Type</th>
    <th>Default</th>
    <th>Description</th>
  </thead>
  <tbody>
    <tr>
      <td>affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Annotations to be added to the deployment</p>
</td>
    </tr>
    <tr>
      <td>config.global.scrape_interval</td>
      <td>string</td>
      <td><pre lang="">
10s
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>config.scrape_configs</td>
      <td>list</td>
      <td><pre lang="plaintext">
- job_name: vmagent
  static_configs:
    - targets:
        - localhost:8429
- bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  job_name: kubernetes-apiservers
  kubernetes_sd_configs:
    - role: endpoints
  relabel_configs:
    - action: keep
      regex: default;kubernetes;https
      source_labels:
        - __meta_kubernetes_namespace
        - __meta_kubernetes_service_name
        - __meta_kubernetes_endpoint_port_name
  scheme: https
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    insecure_skip_verify: true
- bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  job_name: kubernetes-nodes
  kubernetes_sd_configs:
    - role: node
  relabel_configs:
    - action: labelmap
      regex: __meta_kubernetes_node_label_(.+)
    - replacement: kubernetes.default.svc:443
      target_label: __address__
    - regex: (.+)
      replacement: /api/v1/nodes/$1/proxy/metrics
      source_labels:
        - __meta_kubernetes_node_name
      target_label: __metrics_path__
  scheme: https
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    insecure_skip_verify: true
- bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  honor_timestamps: false
  job_name: kubernetes-nodes-cadvisor
  kubernetes_sd_configs:
    - role: node
  relabel_configs:
    - action: labelmap
      regex: __meta_kubernetes_node_label_(.+)
    - replacement: kubernetes.default.svc:443
      target_label: __address__
    - regex: (.+)
      replacement: /api/v1/nodes/$1/proxy/metrics/cadvisor
      source_labels:
        - __meta_kubernetes_node_name
      target_label: __metrics_path__
  scheme: https
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    insecure_skip_verify: true
- job_name: kubernetes-service-endpoints
  kubernetes_sd_configs:
    - role: endpointslices
  relabel_configs:
    - action: drop
      regex: true
      source_labels:
        - __meta_kubernetes_pod_container_init
    - action: keep_if_equal
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_port
        - __meta_kubernetes_pod_container_port_number
    - action: keep
      regex: true
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_scrape
    - action: replace
      regex: (https?)
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_scheme
      target_label: __scheme__
    - action: replace
      regex: (.+)
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_path
      target_label: __metrics_path__
    - action: replace
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2
      source_labels:
        - __address__
        - __meta_kubernetes_service_annotation_prometheus_io_port
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_service_label_(.+)
    - source_labels:
        - __meta_kubernetes_pod_name
      target_label: pod
    - source_labels:
        - __meta_kubernetes_pod_container_name
      target_label: container
    - source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - source_labels:
        - __meta_kubernetes_service_name
      target_label: service
    - replacement: ${1}
      source_labels:
        - __meta_kubernetes_service_name
      target_label: job
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_node_name
      target_label: node
- job_name: kubernetes-service-endpoints-slow
  kubernetes_sd_configs:
    - role: endpointslices
  relabel_configs:
    - action: drop
      regex: true
      source_labels:
        - __meta_kubernetes_pod_container_init
    - action: keep_if_equal
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_port
        - __meta_kubernetes_pod_container_port_number
    - action: keep
      regex: true
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_scrape_slow
    - action: replace
      regex: (https?)
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_scheme
      target_label: __scheme__
    - action: replace
      regex: (.+)
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_path
      target_label: __metrics_path__
    - action: replace
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2
      source_labels:
        - __address__
        - __meta_kubernetes_service_annotation_prometheus_io_port
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_service_label_(.+)
    - source_labels:
        - __meta_kubernetes_pod_name
      target_label: pod
    - source_labels:
        - __meta_kubernetes_pod_container_name
      target_label: container
    - source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - source_labels:
        - __meta_kubernetes_service_name
      target_label: service
    - replacement: ${1}
      source_labels:
        - __meta_kubernetes_service_name
      target_label: job
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_node_name
      target_label: node
  scrape_interval: 5m
  scrape_timeout: 30s
- job_name: kubernetes-services
  kubernetes_sd_configs:
    - role: service
  metrics_path: /probe
  params:
    module:
        - http_2xx
  relabel_configs:
    - action: keep
      regex: true
      source_labels:
        - __meta_kubernetes_service_annotation_prometheus_io_probe
    - source_labels:
        - __address__
      target_label: __param_target
    - replacement: blackbox
      target_label: __address__
    - source_labels:
        - __param_target
      target_label: instance
    - action: labelmap
      regex: __meta_kubernetes_service_label_(.+)
    - source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - source_labels:
        - __meta_kubernetes_service_name
      target_label: service
- job_name: kubernetes-pods
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - action: drop
      regex: true
      source_labels:
        - __meta_kubernetes_pod_container_init
    - action: keep_if_equal
      source_labels:
        - __meta_kubernetes_pod_annotation_prometheus_io_port
        - __meta_kubernetes_pod_container_port_number
    - action: keep
      regex: true
      source_labels:
        - __meta_kubernetes_pod_annotation_prometheus_io_scrape
    - action: replace
      regex: (.+)
      source_labels:
        - __meta_kubernetes_pod_annotation_prometheus_io_path
      target_label: __metrics_path__
    - action: replace
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2
      source_labels:
        - __address__
        - __meta_kubernetes_pod_annotation_prometheus_io_port
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_pod_label_(.+)
    - source_labels:
        - __meta_kubernetes_pod_name
      target_label: pod
    - source_labels:
        - __meta_kubernetes_pod_container_name
      target_label: container
    - source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_node_name
      target_label: node
</pre>
</td>
      <td><p>scrape configuration. scrape self by default</p>
</td>
    </tr>
    <tr>
      <td>configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmagent scraping configuration: <a href="https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/vmagent.md#how-to-collect-metrics-in-prometheus-format" target="_blank">https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/vmagent.md#how-to-collect-metrics-in-prometheus-format</a> use existing configmap if specified otherwise .config values will be used</p>
</td>
    </tr>
    <tr>
      <td>containerWorkingDir</td>
      <td>string</td>
      <td><pre lang="">
/
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>deployment</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: true
strategy: {}
</pre>
</td>
      <td><p>ref: <a href="https://kubernetes.io/docs/concepts/workloads/controllers/deployment/" target="_blank">https://kubernetes.io/docs/concepts/workloads/controllers/deployment/</a></p>
</td>
    </tr>
    <tr>
      <td>emptyDir</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>extraArgs</td>
      <td>object</td>
      <td><pre lang="plaintext">
envflag.enable: "true"
envflag.prefix: VM_
loggerFormat: json
</pre>
</td>
      <td><p>vmagent extra command line arguments</p>
</td>
    </tr>
    <tr>
      <td>extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>extra Labels for Pods, Deployment and Statefulset</p>
</td>
    </tr>
    <tr>
      <td>extraObjects</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Add extra specs dynamically to this chart</p>
</td>
    </tr>
    <tr>
      <td>extraScrapeConfigs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra scrape configs that will be appended to <code>config</code></p>
</td>
    </tr>
    <tr>
      <td>extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>global.compatibility.openshift.adaptSecurityContext</td>
      <td>string</td>
      <td><pre lang="">
auto
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>global.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>global.imagePullSecrets</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>horizontalPodAutoscaling</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
maxReplicas: 10
metrics: []
minReplicas: 1
</pre>
</td>
      <td><p>Horizontal Pod Autoscaling. Note that it is not intended to be used for vmagents which perform scraping. In order to scale scraping vmagents see: <a href="https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets" target="_blank">https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets</a></p>
</td>
    </tr>
    <tr>
      <td>horizontalPodAutoscaling.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Use HPA for vmagent</p>
</td>
    </tr>
    <tr>
      <td>horizontalPodAutoscaling.maxReplicas</td>
      <td>int</td>
      <td><pre lang="">
10
</pre>
</td>
      <td><p>Maximum replicas for HPA to use to to scale vmagent</p>
</td>
    </tr>
    <tr>
      <td>horizontalPodAutoscaling.metrics</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Metric for HPA to use to scale vmagent</p>
</td>
    </tr>
    <tr>
      <td>horizontalPodAutoscaling.minReplicas</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Minimum replicas for HPA to use to scale vmagent</p>
</td>
    </tr>
    <tr>
      <td>image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmagent
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image tag, set to <code>Chart.AppVersion</code> by default</p>
</td>
    </tr>
    <tr>
      <td>image.variant</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Variant of the image to use. e.g. enterprise, scratch</p>
</td>
    </tr>
    <tr>
      <td>imagePullSecrets</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingress.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingress.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>ingress.tls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>license</td>
      <td>object</td>
      <td><pre lang="plaintext">
key: ""
secret:
    key: ""
    name: ""
</pre>
</td>
      <td><p>Enterprise license key configuration for VictoriaMetrics enterprise. Required only for VictoriaMetrics enterprise. Documentation - <a href="https://docs.victoriametrics.com/enterprise" target="_blank">https://docs.victoriametrics.com/enterprise</a>, for more information, visit <a href="https://victoriametrics.com/products/enterprise/" target="_blank">https://victoriametrics.com/products/enterprise/</a> . To request a trial license, go to <a href="https://victoriametrics.com/products/enterprise/trial/" target="_blank">https://victoriametrics.com/products/enterprise/trial/</a> Supported starting from VictoriaMetrics v1.94.0</p>
</td>
    </tr>
    <tr>
      <td>license.key</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>License key</p>
</td>
    </tr>
    <tr>
      <td>license.secret</td>
      <td>object</td>
      <td><pre lang="plaintext">
key: ""
name: ""
</pre>
</td>
      <td><p>Use existing secret with license key</p>
</td>
    </tr>
    <tr>
      <td>license.secret.key</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Key in secret with license key</p>
</td>
    </tr>
    <tr>
      <td>license.secret.name</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Existing secret name</p>
</td>
    </tr>
    <tr>
      <td>multiTenantUrls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>nameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.accessModes[0]</td>
      <td>string</td>
      <td><pre lang="">
ReadWriteOnce
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.existingClaim</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>persistence.matchLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Bind Persistent Volume by labels. Must match all labels of targeted PV.</p>
</td>
    </tr>
    <tr>
      <td>persistence.size</td>
      <td>string</td>
      <td><pre lang="">
10Gi
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Annotations to be added to pod</p>
</td>
    </tr>
    <tr>
      <td>podDisruptionBudget</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
labels: {}
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more ref: <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">https://kubernetes.io/docs/tasks/run-application/configure-pdb/</a></p>
</td>
    </tr>
    <tr>
      <td>podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>extra Labels for Pods only</p>
</td>
    </tr>
    <tr>
      <td>podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>priority class to be assigned to the pod(s)</p>
</td>
    </tr>
    <tr>
      <td>probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
initialDelaySeconds: 5
periodSeconds: 15
tcpSocket: {}
timeoutSeconds: 5
</pre>
</td>
      <td><p>liveness probe</p>
</td>
    </tr>
    <tr>
      <td>probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
</pre>
</td>
      <td><p>readiness probe</p>
</td>
    </tr>
    <tr>
      <td>probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>startup probe</p>
</td>
    </tr>
    <tr>
      <td>rbac.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>rbac.create</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>rbac.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>rbac.namespaced</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>if true and <code>rbac.enabled</code>, will deploy a Role/Rolebinding instead of a ClusterRole/ClusterRoleBinding</p>
</td>
    </tr>
    <tr>
      <td>remoteWriteUrls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>WARN: need to specify at least one remote write url or one multi tenant url</p>
</td>
    </tr>
    <tr>
      <td>replicaCount</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>securityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8429
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>serviceAccount.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Annotations to add to the service account</p>
</td>
    </tr>
    <tr>
      <td>serviceAccount.create</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Specifies whether a service account should be created</p>
</td>
    </tr>
    <tr>
      <td>serviceAccount.name</td>
      <td>string</td>
      <td><pre lang="">
null
</pre>
</td>
      <td><p>The name of the service account to use. If not set and create is true, a name is generated using the fullname template</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for server component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>statefulset</td>
      <td>object</td>
      <td><pre lang="plaintext">
clusterMode: false
enabled: false
replicationFactor: 1
updateStrategy: {}
</pre>
</td>
      <td><p>ref: <a href="https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/" target="_blank">https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/</a></p>
</td>
    </tr>
    <tr>
      <td>statefulset.clusterMode</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>create cluster of vmagents. See <a href="https://docs.victoriametrics.com/vmagent#scraping-big-number-of-targets" target="_blank">https://docs.victoriametrics.com/vmagent#scraping-big-number-of-targets</a> available since 1.77.2 version <a href="https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.77.2" target="_blank">https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.77.2</a></p>
</td>
    </tr>
    <tr>
      <td>statefulset.replicationFactor</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>replication factor for vmagent in cluster mode</p>
</td>
    </tr>
    <tr>
      <td>tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>topologySpreadConstraints</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
  </tbody>
</table>

