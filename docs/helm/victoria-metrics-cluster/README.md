
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.13.7](https://img.shields.io/badge/Version-0.13.7-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-cluster)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Metrics Cluster version - high-performance, cost-effective and scalable TSDB, long-term remote storage for Prometheus

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](../../REQUIREMENTS.md).

* PV support on underlying infrastructure

## Chart Details

Note: this chart installs VictoriaMetrics cluster components such as vminsert, vmselect and vmstorage. It doesn't create or configure metrics scraping. If you are looking for a chart to configure monitoring stack in cluster check out [victoria-metrics-k8s-stack chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-k8s-stack#helm-chart-for-victoria-metrics-kubernetes-monitoring-stack).

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

List versions of ``vm/victoria-metrics-cluster`` chart available to installation:

```console
helm search repo vm/victoria-metrics-cluster -l
```

Export default values of ``victoria-metrics-cluster`` chart to file ``values.yaml``:

```console
helm show values vm/victoria-metrics-cluster > values.yaml
```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

```console
helm install vmcluster vm/victoria-metrics-cluster -f values.yaml -n NAMESPACE --debug --dry-run
```

Install chart with command:

```console
helm install vmcluster vm/victoria-metrics-cluster -f values.yaml -n NAMESPACE
```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'vminsert\|vmselect\|vmstorage'
```

Get the application by running this command:

```console
helm list -f vmcluster -n NAMESPACE
```

See the history of versions of ``vmcluster`` application with command.

```console
helm history vmcluster -n NAMESPACE
```

## How to uninstall

Remove application with command.

```console
helm uninstall vmcluster -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](../../REQUIREMENTS.md).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-cluster

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-cluster/values.yaml`` file.

<table>
  <thead>
    <th>Key</th>
    <th>Type</th>
    <th>Default</th>
    <th>Description</th>
  </thead>
  <tbody>
    <tr>
      <td>clusterDomainSuffix</td>
      <td>string</td>
      <td><pre lang="">
cluster.local
</pre>
</td>
      <td><p>k8s cluster domain suffix, uses for building storage pods&rsquo; FQDN. Details are <a href="https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/" target="_blank">here</a></p>
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
      <td>extraSecrets</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
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
      <td>printNotes</td>
      <td>bool</td>
      <td><pre lang="">
true
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
      <td></td>
    </tr>
    <tr>
      <td>serviceAccount.automountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>serviceAccount.create</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
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
    <tr>
      <td>vminsert.affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod affinity</p>
</td>
    </tr>
    <tr>
      <td>vminsert.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.automountServiceAccountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.containerWorkingDir</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Container workdir</p>
</td>
    </tr>
    <tr>
      <td>vminsert.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Enable deployment of vminsert component. Deployment is used</p>
</td>
    </tr>
    <tr>
      <td>vminsert.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Overrides the full name of vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.horizontalPodAutoscaler.behavior</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Behavior settings for scaling by the HPA</p>
</td>
    </tr>
    <tr>
      <td>vminsert.horizontalPodAutoscaler.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Use HPA for vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.horizontalPodAutoscaler.maxReplicas</td>
      <td>int</td>
      <td><pre lang="">
10
</pre>
</td>
      <td><p>Maximum replicas for HPA to use to to scale the vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.horizontalPodAutoscaler.metrics</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Metric for HPA to use to scale the vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.horizontalPodAutoscaler.minReplicas</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>Minimum replicas for HPA to use to scale the vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td><p>Image pull policy</p>
</td>
    </tr>
    <tr>
      <td>vminsert.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>vminsert.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vminsert
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>vminsert.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image tag override Chart.AppVersion</p>
</td>
    </tr>
    <tr>
      <td>vminsert.image.variant</td>
      <td>string</td>
      <td><pre lang="">
cluster
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.ingress.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Ingress annotations</p>
</td>
    </tr>
    <tr>
      <td>vminsert.ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of ingress for vminsert component</p>
</td>
    </tr>
    <tr>
      <td>vminsert.ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of host objects</p>
</td>
    </tr>
    <tr>
      <td>vminsert.ingress.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>vminsert.ingress.tls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of TLS objects</p>
</td>
    </tr>
    <tr>
      <td>vminsert.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.name</td>
      <td>string</td>
      <td><pre lang="">
vminsert
</pre>
</td>
      <td><p>vminsert container name</p>
</td>
    </tr>
    <tr>
      <td>vminsert.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s node selector. Details are <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s annotations</p>
</td>
    </tr>
    <tr>
      <td>vminsert.podDisruptionBudget.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Details are <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.podDisruptionBudget.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.ports.name</td>
      <td>string</td>
      <td><pre lang="">
http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Name of Priority Class</p>
</td>
    </tr>
    <tr>
      <td>vminsert.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
initialDelaySeconds: 5
periodSeconds: 15
tcpSocket: {}
timeoutSeconds: 5
</pre>
</td>
      <td><p>vminsert liveness probe</p>
</td>
    </tr>
    <tr>
      <td>vminsert.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</pre>
</td>
      <td><p>vminsert readiness probe</p>
</td>
    </tr>
    <tr>
      <td>vminsert.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>vminsert startup probe</p>
</td>
    </tr>
    <tr>
      <td>vminsert.replicaCount</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>Count of vminsert pods</p>
</td>
    </tr>
    <tr>
      <td>vminsert.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Resource object</p>
</td>
    </tr>
    <tr>
      <td>vminsert.securityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
</pre>
</td>
      <td><p>Pod&rsquo;s security context. Details are <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service annotations</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service ClusterIP</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service External IPs. Details are <a href="https://kubernetes.io/docs/user-guide/services/#external-ips" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.service.extraPorts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra service ports</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.service.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service labels</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service load balancer IP</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Load balancer source range</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8480
</pre>
</td>
      <td><p>Service port</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.targetPort</td>
      <td>string</td>
      <td><pre lang="">
http
</pre>
</td>
      <td><p>Target port</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td><p>Service type</p>
</td>
    </tr>
    <tr>
      <td>vminsert.service.udp</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Make sure that service is not type &ldquo;LoadBalancer&rdquo;, as it requires &ldquo;MixedProtocolLBService&rdquo; feature gate. ref: <a href="https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/" target="_blank">https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for vminsert component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.namespace</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Target namespace of ServiceMonitor manifest</p>
</td>
    </tr>
    <tr>
      <td>vminsert.serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>vminsert.strategy</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vminsert.suppressStorageFQDNsRender</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Suppress rendering <code>--storageNode</code> FQDNs based on <code>vmstorage.replicaCount</code> value. If true suppress rendering <code>--storageNodes</code>, they can be re-defined in extraArgs</p>
</td>
    </tr>
    <tr>
      <td>vminsert.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of tolerations object. Details are <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vminsert.topologySpreadConstraints</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Pod topologySpreadConstraints</p>
</td>
    </tr>
    <tr>
      <td>vmselect.affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod affinity</p>
</td>
    </tr>
    <tr>
      <td>vmselect.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.automountServiceAccountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.cacheMountPath</td>
      <td>string</td>
      <td><pre lang="">
/cache
</pre>
</td>
      <td><p>Cache root folder</p>
</td>
    </tr>
    <tr>
      <td>vmselect.containerWorkingDir</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Container workdir</p>
</td>
    </tr>
    <tr>
      <td>vmselect.emptyDir</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Enable deployment of vmselect component. Can be deployed as Deployment(default) or StatefulSet</p>
</td>
    </tr>
    <tr>
      <td>vmselect.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraArgs."envflag.enable"</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>vmselect.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>vmselect.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>vmselect.fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Overrides the full name of vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.horizontalPodAutoscaler.behavior</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Behavior settings for scaling by the HPA</p>
</td>
    </tr>
    <tr>
      <td>vmselect.horizontalPodAutoscaler.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Use HPA for vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.horizontalPodAutoscaler.maxReplicas</td>
      <td>int</td>
      <td><pre lang="">
10
</pre>
</td>
      <td><p>Maximum replicas for HPA to use to to scale the vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.horizontalPodAutoscaler.metrics</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Metric for HPA to use to scale the vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.horizontalPodAutoscaler.minReplicas</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>Minimum replicas for HPA to use to scale the vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td><p>Image pull policy</p>
</td>
    </tr>
    <tr>
      <td>vmselect.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>vmselect.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmselect
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>vmselect.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image tag override Chart.AppVersion</p>
</td>
    </tr>
    <tr>
      <td>vmselect.image.variant</td>
      <td>string</td>
      <td><pre lang="">
cluster
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.ingress.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Ingress annotations</p>
</td>
    </tr>
    <tr>
      <td>vmselect.ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of ingress for vmselect component</p>
</td>
    </tr>
    <tr>
      <td>vmselect.ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of host objects</p>
</td>
    </tr>
    <tr>
      <td>vmselect.ingress.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>vmselect.ingress.tls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of TLS objects</p>
</td>
    </tr>
    <tr>
      <td>vmselect.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.name</td>
      <td>string</td>
      <td><pre lang="">
vmselect
</pre>
</td>
      <td><p>Vmselect container name</p>
</td>
    </tr>
    <tr>
      <td>vmselect.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s node selector. Details are <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.accessModes</td>
      <td>list</td>
      <td><pre lang="plaintext">
- ReadWriteOnce
</pre>
</td>
      <td><p>Array of access mode. Must match those of existing PV or dynamic provisioner. Details are <a href="http://kubernetes.io/docs/user-guide/persistent-volumes/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistent volume annotations</p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Create/use Persistent Volume Claim for vmselect component. Empty dir if false. If true, vmselect will create/use a Persistent Volume Claim</p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.existingClaim</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Existing Claim name. Requires vmselect.persistentVolume.enabled: true. If defined, PVC must be created manually before volume will be bound</p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistent volume labels</p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.size</td>
      <td>string</td>
      <td><pre lang="">
2Gi
</pre>
</td>
      <td><p>Size of the volume. Better to set the same as resource limit memory property</p>
</td>
    </tr>
    <tr>
      <td>vmselect.persistentVolume.subPath</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Mount subpath</p>
</td>
    </tr>
    <tr>
      <td>vmselect.podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s annotations</p>
</td>
    </tr>
    <tr>
      <td>vmselect.podDisruptionBudget.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Details are <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.podDisruptionBudget.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.ports.name</td>
      <td>string</td>
      <td><pre lang="">
http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Name of Priority Class</p>
</td>
    </tr>
    <tr>
      <td>vmselect.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
initialDelaySeconds: 5
periodSeconds: 15
tcpSocket: {}
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmselect liveness probe</p>
</td>
    </tr>
    <tr>
      <td>vmselect.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmselect readiness probe</p>
</td>
    </tr>
    <tr>
      <td>vmselect.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>vmselect startup probe</p>
</td>
    </tr>
    <tr>
      <td>vmselect.replicaCount</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>Count of vmselect pods</p>
</td>
    </tr>
    <tr>
      <td>vmselect.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Resource object</p>
</td>
    </tr>
    <tr>
      <td>vmselect.securityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: true
</pre>
</td>
      <td><p>Pod&rsquo;s security context. Details are <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service annotations</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service ClusterIP</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service External IPs. Details are <a href="https://kubernetes.io/docs/user-guide/services/#external-ips" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.service.extraPorts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra service ports</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.service.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service labels</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Service load balacner IP</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Load balancer source range</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8481
</pre>
</td>
      <td><p>Service port</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.targetPort</td>
      <td>string</td>
      <td><pre lang="">
http
</pre>
</td>
      <td><p>Target port</p>
</td>
    </tr>
    <tr>
      <td>vmselect.service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td><p>Service type</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for vmselect component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.namespace</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Target namespace of ServiceMonitor manifest</p>
</td>
    </tr>
    <tr>
      <td>vmselect.serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>vmselect.statefulSet.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Deploy StatefulSet instead of Deployment for vmselect. Useful if you want to keep cache data.</p>
</td>
    </tr>
    <tr>
      <td>vmselect.statefulSet.podManagementPolicy</td>
      <td>string</td>
      <td><pre lang="">
OrderedReady
</pre>
</td>
      <td><p>Deploy order policy for StatefulSet pods</p>
</td>
    </tr>
    <tr>
      <td>vmselect.strategy</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmselect.suppressStorageFQDNsRender</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Suppress rendering <code>--storageNode</code> FQDNs based on <code>vmstorage.replicaCount</code> value. If true suppress rendering <code>--storageNodes</code>, they can be re-defined in extraArgs</p>
</td>
    </tr>
    <tr>
      <td>vmselect.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of tolerations object. Details are <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmselect.topologySpreadConstraints</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Pod topologySpreadConstraints</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod affinity</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.automountServiceAccountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.containerWorkingDir</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Container workdir</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.emptyDir</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Use an alternate scheduler, e.g. &ldquo;stork&rdquo;. ref: <a href="https://kubernetes.io/docs/tasks/administer-cluster/configure-multiple-schedulers/" target="_blank">https://kubernetes.io/docs/tasks/administer-cluster/configure-multiple-schedulers/</a>  schedulerName:</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Enable deployment of vmstorage component. StatefulSet is used</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraSecretMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
null
</pre>
</td>
      <td><p>Overrides the full name of vmstorage component</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td><p>Image pull policy</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmstorage
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Image tag override Chart.AppVersion</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.image.variant</td>
      <td>string</td>
      <td><pre lang="">
cluster
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.name</td>
      <td>string</td>
      <td><pre lang="">
vmstorage
</pre>
</td>
      <td><p>vmstorage container name</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s node selector. Details are <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.accessModes</td>
      <td>list</td>
      <td><pre lang="plaintext">
- ReadWriteOnce
</pre>
</td>
      <td><p>Array of access modes. Must match those of existing PV or dynamic provisioner. Details are <a href="http://kubernetes.io/docs/user-guide/persistent-volumes/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistent volume annotations</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>Create/use Persistent Volume Claim for vmstorage component. Empty dir if false. If true,  vmstorage will create/use a Persistent Volume Claim</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.existingClaim</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Existing Claim name. Requires vmstorage.persistentVolume.enabled: true. If defined, PVC must be created manually before volume will be bound</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistent volume labels</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.mountPath</td>
      <td>string</td>
      <td><pre lang="">
/storage
</pre>
</td>
      <td><p>Data root path. Vmstorage data Persistent Volume mount root path</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.name</td>
      <td>string</td>
      <td><pre lang="">
vmstorage-volume
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.size</td>
      <td>string</td>
      <td><pre lang="">
8Gi
</pre>
</td>
      <td><p>Size of the volume.</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.storageClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Storage class name. Will be empty if not setted</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.persistentVolume.subPath</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Mount subpath</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Pod&rsquo;s annotations</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.podDisruptionBudget</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
labels: {}
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Details are <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.podManagementPolicy</td>
      <td>string</td>
      <td><pre lang="">
OrderedReady
</pre>
</td>
      <td><p>Deploy order policy for StatefulSet pods</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.ports.name</td>
      <td>string</td>
      <td><pre lang="">
http
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Name of Priority Class</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 10
initialDelaySeconds: 30
periodSeconds: 30
tcpSocket: {}
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmstorage liveness probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmstorage readiness probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>vmstorage startup probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.replicaCount</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>Count of vmstorage pods</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Resource object. Details are <a href="https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.retentionPeriod</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Data retention period. Supported values 1w, 1d, number without measurement means month, e.g. 2 = 2month</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.securityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
</pre>
</td>
      <td><p>Pod&rsquo;s security context. Details are <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service annotations</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.service.extraPorts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra service ports</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.service.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service labels</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8482
</pre>
</td>
      <td><p>Service port</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.vminsertPort</td>
      <td>int</td>
      <td><pre lang="">
8400
</pre>
</td>
      <td><p>Port for accepting connections from vminsert</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.service.vmselectPort</td>
      <td>int</td>
      <td><pre lang="">
8401
</pre>
</td>
      <td><p>Port for accepting connections from vmselect</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for vmstorage component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.namespace</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Target namespace of ServiceMonitor manifest</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.terminationGracePeriodSeconds</td>
      <td>int</td>
      <td><pre lang="">
60
</pre>
</td>
      <td><p>Pod&rsquo;s termination grace period in seconds</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Array of tolerations object. Node tolerations for server scheduling to nodes with taints. Details are <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">here</a> #</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.topologySpreadConstraints</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Pod topologySpreadConstraints</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.destination</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>backup destination at S3, GCS or local filesystem. Pod name will be included to path!</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.disableDaily</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable daily backups</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.disableHourly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable hourly backups</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.disableMonthly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable monthly backups</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.disableWeekly</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>disable weekly backups</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>enable automatic creation of backup via vmbackupmanager. vmbackupmanager is part of Enterprise packages</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.env</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://docs.victoriametrics.com/#environment-variables" target="_blank">https://docs.victoriametrics.com/#environment-variables</a></p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.extraSecretMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.image.registry</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmbackupmanager image registry</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmbackupmanager
</pre>
</td>
      <td><p>vmbackupmanager image repository</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmbackupmanager image tag override Chart.AppVersion</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.image.variant</td>
      <td>string</td>
      <td><pre lang="">
cluster
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 10
initialDelaySeconds: 30
periodSeconds: 30
tcpSocket:
    port: manager-http
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmbackupmanager liveness probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
httpGet:
    port: manager-http
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</pre>
</td>
      <td><p>vmbackupmanager readiness probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>vmbackupmanager startup probe</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.restore</td>
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
      <td>vmstorage.vmbackupmanager.retention</td>
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
      <td>vmstorage.vmbackupmanager.retention.keepLastDaily</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N daily backups. 0 means delete all existing daily backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.retention.keepLastHourly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N hourly backups. 0 means delete all existing hourly backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.retention.keepLastMonthly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N monthly backups. 0 means delete all existing monthly backups. Specify -1 to turn off</p>
</td>
    </tr>
    <tr>
      <td>vmstorage.vmbackupmanager.retention.keepLastWeekly</td>
      <td>int</td>
      <td><pre lang="">
2
</pre>
</td>
      <td><p>keep last N weekly backups. 0 means delete all existing weekly backups. Specify -1 to turn off</p>
</td>
    </tr>
  </tbody>
</table>

