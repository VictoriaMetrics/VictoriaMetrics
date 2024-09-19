
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.11.2](https://img.shields.io/badge/Version-0.11.2-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-single)

Victoria Metrics Single version - high-performance, cost-effective and scalable TSDB, long-term remote storage for Prometheus

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](../../REQUIREMENTS.md).
* PV support on underlying infrastructure.

## Chart Details

This chart will do the following:

* Rollout Victoria Metrics Single.

## How to install

Access a Kubernetes cluster.

Add a chart helm repository with follow commands:

 - From HTTPS repository

   ```console
   helm repo add vm https://victoriametrics.github.io/helm-charts/

   helm repo update
   ```
 - From OCI repository
  
   ```console
   helm repo add vm oci://ghcr.io/victoriametrics/helm-charts/

   helm repo update
   ```

List versions of ``vm/victoria-metrics-single`` chart available to installation:

```console
helm search repo vm/victoria-metrics-single -l
```

Export default values of ``victoria-metrics-single`` chart to file ``values.yaml``:

```console
helm show values vm/victoria-metrics-single > values.yaml
```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

```console
helm install vmsingle vm/victoria-metrics-single -f values.yaml -n NAMESPACE --debug --dry-run
```

Install chart with command:

```console
helm install vmsingle vm/victoria-metrics-single -f values.yaml -n NAMESPACE
```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'single'
```

Get the application by running this command:

```console
helm list -f vmsingle -n NAMESPACE
```

See the history of versions of ``vmsingle`` application with command.

```console
helm history vmsingle -n NAMESPACE
```

## How to uninstall

Remove application with command.

```console
helm uninstall vmsingle -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](../../REQUIREMENTS.md).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-single

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-single/values.yaml`` file.

<table>
  <thead>
    <th>Key</th>
    <th>Type</th>
    <th>Default</th>
    <th>Description</th>
  </thead>
  <tbody>
    <tr>
      <td>automountServiceAccountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
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
      <td>podDisruptionBudget.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Details are <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>podDisruptionBudget.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>printNotes</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Print chart notes</p>
</td>
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
      <td></td>
    </tr>
    <tr>
      <td>server.affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod affinity</p>
</td>
    </tr>
    <tr>
      <td>server.containerWorkingDir</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Container workdir</p>
</td>
    </tr>
    <tr>
      <td>server.emptyDir</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Enable deployment of server component. Deployed as StatefulSet</p>
</td>
    </tr>
    <tr>
      <td>server.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>server.envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Sts/Deploy additional labels</p>
</td>
    </tr>
    <tr>
      <td>server.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
null
</pre>
</td>
      <td><p>Overrides the full name of server component</p>
</td>
    </tr>
    <tr>
      <td>server.image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td><p>Image pull policy</p>
</td>
    </tr>
    <tr>
      <td>server.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>server.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/victoria-metrics
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>server.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image tag</p>
</td>
    </tr>
    <tr>
      <td>server.image.variant</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.imagePullSecrets</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.ingress.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Ingress annotations</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of ingress for server component</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Ingress extra labels</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of host objects</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.tls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of TLS objects</p>
</td>
    </tr>
    <tr>
      <td>server.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.name</td>
      <td>string</td>
      <td><pre lang="">
server
</pre>
</td>
      <td><p>Server container name</p>
</td>
    </tr>
    <tr>
      <td>server.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s node selector. Details are <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.accessModes</td>
      <td>list</td>
      <td><pre lang="plaintext">
- ReadWriteOnce
</pre>
</td>
      <td><p>Array of access modes. Must match those of existing PV or dynamic provisioner. Details are <a href="http://kubernetes.io/docs/user-guide/persistent-volumes/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistant volume annotations</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Create/use Persistent Volume Claim for server component. Empty dir if false</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.existingClaim</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Existing Claim name. If defined, PVC must be created manually before volume will be bound</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.matchLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Bind Persistent Volume by labels. Must match all labels of targeted PV.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.mountPath</td>
      <td>string</td>
      <td><pre lang="">
/storage
</pre>
</td>
      <td><p>Mount path. Server data Persistent Volume mount root path.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.size</td>
      <td>string</td>
      <td><pre lang="">
16Gi
</pre>
</td>
      <td><p>Size of the volume. Should be calculated based on the metrics you send and retention policy you set.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.storageClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>StorageClass to use for persistent volume. Requires server.persistentVolume.enabled: true. If defined, PVC created automatically</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.subPath</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Mount subpath</p>
</td>
    </tr>
    <tr>
      <td>server.podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s annotations</p>
</td>
    </tr>
    <tr>
      <td>server.podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s additional labels</p>
</td>
    </tr>
    <tr>
      <td>server.podManagementPolicy</td>
      <td>string</td>
      <td><pre lang="">
OrderedReady
</pre>
</td>
      <td><p>Pod&rsquo;s management policy</p>
</td>
    </tr>
    <tr>
      <td>server.podSecurityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: true
</pre>
</td>
      <td><p>Pod&rsquo;s security context. Details are <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Name of Priority Class</p>
</td>
    </tr>
    <tr>
      <td>server.probe.liveness.failureThreshold</td>
      <td>int</td>
      <td><pre lang="">
10
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.liveness.initialDelaySeconds</td>
      <td>int</td>
      <td><pre lang="">
30
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.liveness.periodSeconds</td>
      <td>int</td>
      <td><pre lang="">
30
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.liveness.tcpSocket</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.liveness.timeoutSeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.readiness.failureThreshold</td>
      <td>int</td>
      <td><pre lang="">
3
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.readiness.httpGet</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.readiness.initialDelaySeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.readiness.periodSeconds</td>
      <td>int</td>
      <td><pre lang="">
15
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.readiness.timeoutSeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.relabel</td>
      <td>object</td>
      <td><pre lang="plaintext">
config: []
configMap: ""
enabled: false
</pre>
</td>
      <td><p>Global relabel configuration</p>
</td>
    </tr>
    <tr>
      <td>server.relabel.configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Use existing configmap if specified otherwise .config values will be used. Relabel config <strong>should</strong> reside under <code>relabel.yml</code> key</p>
</td>
    </tr>
    <tr>
      <td>server.replicaCount</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Replica count</p>
</td>
    </tr>
    <tr>
      <td>server.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Resource object. Details are <a href="http://kubernetes.io/docs/user-guide/compute-resources/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.retentionPeriod</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Data retention period in month</p>
</td>
    </tr>
    <tr>
      <td>server.scrape</td>
      <td>object</td>
      <td><pre lang="plaintext">
config:
    global:
        scrape_interval: 15s
    scrape_configs:
        - job_name: victoriametrics
          static_configs:
            - targets:
                - localhost:8428
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
            - role: endpoints
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
            - action: replace
              source_labels:
                - __meta_kubernetes_namespace
              target_label: namespace
            - action: replace
              source_labels:
                - __meta_kubernetes_service_name
              target_label: service
            - action: replace
              source_labels:
                - __meta_kubernetes_pod_node_name
              target_label: node
        - job_name: kubernetes-service-endpoints-slow
          kubernetes_sd_configs:
            - role: endpoints
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
            - action: replace
              source_labels:
                - __meta_kubernetes_namespace
              target_label: namespace
            - action: replace
              source_labels:
                - __meta_kubernetes_service_name
              target_label: service
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
            - action: replace
              source_labels:
                - __meta_kubernetes_namespace
              target_label: namespace
            - action: replace
              source_labels:
                - __meta_kubernetes_pod_name
              target_label: pod
configMap: ""
enabled: false
extraScrapeConfigs: []
</pre>
</td>
      <td><p>Scrape configuration for victoriametrics</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.config</td>
      <td>object</td>
      <td><pre lang="plaintext">
global:
    scrape_interval: 15s
scrape_configs:
    - job_name: victoriametrics
      static_configs:
        - targets:
            - localhost:8428
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
        - role: endpoints
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
        - action: replace
          source_labels:
            - __meta_kubernetes_namespace
          target_label: namespace
        - action: replace
          source_labels:
            - __meta_kubernetes_service_name
          target_label: service
        - action: replace
          source_labels:
            - __meta_kubernetes_pod_node_name
          target_label: node
    - job_name: kubernetes-service-endpoints-slow
      kubernetes_sd_configs:
        - role: endpoints
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
        - action: replace
          source_labels:
            - __meta_kubernetes_namespace
          target_label: namespace
        - action: replace
          source_labels:
            - __meta_kubernetes_service_name
          target_label: service
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
        - action: replace
          source_labels:
            - __meta_kubernetes_namespace
          target_label: namespace
        - action: replace
          source_labels:
            - __meta_kubernetes_pod_name
          target_label: pod
</pre>
</td>
      <td><p>Scrape config</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.config.scrape_configs</td>
      <td>list</td>
      <td><pre lang="plaintext">
- job_name: victoriametrics
  static_configs:
    - targets:
        - localhost:8428
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
    - role: endpoints
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
    - action: replace
      source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - action: replace
      source_labels:
        - __meta_kubernetes_service_name
      target_label: service
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_node_name
      target_label: node
- job_name: kubernetes-service-endpoints-slow
  kubernetes_sd_configs:
    - role: endpoints
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
    - action: replace
      source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - action: replace
      source_labels:
        - __meta_kubernetes_service_name
      target_label: service
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
    - action: replace
      source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_name
      target_label: pod
</pre>
</td>
      <td><p>Scrape targets</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.config.scrape_configs[4]</td>
      <td>object</td>
      <td><pre lang="plaintext">
job_name: kubernetes-service-endpoints
kubernetes_sd_configs:
    - role: endpoints
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
    - action: replace
      source_labels:
        - __meta_kubernetes_namespace
      target_label: namespace
    - action: replace
      source_labels:
        - __meta_kubernetes_service_name
      target_label: service
    - action: replace
      source_labels:
        - __meta_kubernetes_pod_node_name
      target_label: node
</pre>
</td>
      <td><p>Scrape rule using kubernetes service discovery for endpoints</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.config.scrape_configs[6]</td>
      <td>object</td>
      <td><pre lang="plaintext">
job_name: kubernetes-services
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
</pre>
</td>
      <td><p>Example scrape config for probing services via the Blackbox Exporter.  The relabeling allows the actual service scrape endpoint to be configured via the following annotations:  * <code>prometheus.io/probe</code>: Only probe services that have a value of <code>true</code></p>
</td>
    </tr>
    <tr>
      <td>server.scrape.configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Use existing configmap if specified otherwise .config values will be used. Scrape config <strong>should</strong> reside under <code>scrape.yml</code> key</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>If true scrapes targets, creates config map or use specified one with scrape targets</p>
</td>
    </tr>
    <tr>
      <td>server.scrape.extraScrapeConfigs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra scrape configs that will be appended to <code>server.scrape.config</code></p>
</td>
    </tr>
    <tr>
      <td>server.securityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: true
</pre>
</td>
      <td><p>Security context to be added to server pods</p>
</td>
    </tr>
    <tr>
      <td>server.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service annotations</p>
</td>
    </tr>
    <tr>
      <td>server.service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service ClusterIP</p>
</td>
    </tr>
    <tr>
      <td>server.service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service External IPs. Details are <a href="https://kubernetes.io/docs/user-guide/services/#external-ips" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service labels</p>
</td>
    </tr>
    <tr>
      <td>server.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service load balacner IP</p>
</td>
    </tr>
    <tr>
      <td>server.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Load balancer source range</p>
</td>
    </tr>
    <tr>
      <td>server.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8428
</pre>
</td>
      <td><p>Service port</p>
</td>
    </tr>
    <tr>
      <td>server.service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td><p>Service type</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for server component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>server.statefulSet.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Creates statefulset instead of deployment, useful when you want to keep the cache</p>
</td>
    </tr>
    <tr>
      <td>server.statefulSet.podManagementPolicy</td>
      <td>string</td>
      <td><pre lang="">
OrderedReady
</pre>
</td>
      <td><p>Deploy order policy for StatefulSet pods</p>
</td>
    </tr>
    <tr>
      <td>server.terminationGracePeriodSeconds</td>
      <td>int</td>
      <td><pre lang="">
60
</pre>
</td>
      <td><p>Pod&rsquo;s termination grace period in seconds</p>
</td>
    </tr>
    <tr>
      <td>server.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Node tolerations for server scheduling to nodes with taints. Details are <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.destination</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>backup destination at S3, GCS or local filesystem. Release name will be included to path!</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.disableDaily</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable daily backups</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.disableHourly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable hourly backups</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.disableMonthly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable monthly backups</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.disableWeekly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable weekly backups</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>enable automatic creation of backup via vmbackupmanager. vmbackupmanager is part of Enterprise packages</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmbackupmanager image registry</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmbackupmanager
</pre>
</td>
      <td><p>vmbackupmanager image repository</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmbackupmanager image tag</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.image.variant</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.liveness.failureThreshold</td>
      <td>int</td>
      <td><pre lang="">
10
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.liveness.initialDelaySeconds</td>
      <td>int</td>
      <td><pre lang="">
30
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.liveness.periodSeconds</td>
      <td>int</td>
      <td><pre lang="">
30
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.liveness.tcpSocket.port</td>
      <td>string</td>
      <td><pre lang="">
manager-http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.liveness.timeoutSeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.readiness.failureThreshold</td>
      <td>int</td>
      <td><pre lang="">
3
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.readiness.httpGet.port</td>
      <td>string</td>
      <td><pre lang="">
manager-http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.readiness.initialDelaySeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.readiness.periodSeconds</td>
      <td>int</td>
      <td><pre lang="">
15
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.readiness.timeoutSeconds</td>
      <td>int</td>
      <td><pre lang="">
5
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.probe.startup.httpGet.port</td>
      <td>string</td>
      <td><pre lang="">
manager-http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.restore</td>
      <td>object</td>
      <td><pre lang="plaintext">
onStart:
    enabled: false
</pre>
</td>
      <td><p>Allows to enable restore options for pod. Read more: <a href="https://docs.victoriametrics.com/vmbackupmanager#restore-commands" target="_blank">https://docs.victoriametrics.com/vmbackupmanager#restore-commands</a></p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.retention</td>
      <td>object</td>
      <td><pre lang="plaintext">
keepLastDaily: 2
keepLastHourly: 2
keepLastMonthly: 2
keepLastWeekly: 2
</pre>
</td>
      <td><p>backups&rsquo; retention settings</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.retention.keepLastDaily</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N daily backups. 0 means delete all existing daily backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.retention.keepLastHourly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N hourly backups. 0 means delete all existing hourly backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.retention.keepLastMonthly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N monthly backups. 0 means delete all existing monthly backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>server.vmbackupmanager.retention.keepLastWeekly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N weekly backups. 0 means delete all existing weekly backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>serviceAccount.automountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Mount API token to pod directly</p>
</td>
    </tr>
    <tr>
      <td>serviceAccount.create</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Create service account.</p>
</td>
    </tr>
    <tr>
      <td>serviceAccount.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
  </tbody>
</table>

