
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.11.1](https://img.shields.io/badge/Version-0.11.1-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-alert)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Metrics Alert - executes a list of given MetricsQL expressions (rules) and sends alerts to Alert Manager.

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](../../REQUIREMENTS.md).

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

List versions of ``vm/victoria-metrics-alert`` chart available to installation:

```console
helm search repo vm/victoria-metrics-alert -l
```

Export default values of ``victoria-metrics-alert`` chart to file ``values.yaml``:

```console
helm show values vm/victoria-metrics-alert > values.yaml
```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

```console
helm install vmalert vm/victoria-metrics-alert -f values.yaml -n NAMESPACE --debug --dry-run
```

Install chart with command:

```console
helm install vmalert vm/victoria-metrics-alert -f values.yaml -n NAMESPACE
```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'alert'
```

Get the application by running this command:

```console
helm list -f vmalert -n NAMESPACE
```

See the history of versions of ``vmalert`` application with command.

```console
helm history vmalert -n NAMESPACE
```

## HA configuration for Alertmanager

There is no option on this chart to set up Alertmanager with [HA mode](https://github.com/prometheus/alertmanager#high-availability).
To enable the HA configuration, you can use:
- [VictoriaMetrics Operator](https://docs.victoriametrics.com/operator/)
- official [Alertmanager Helm chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/alertmanager)

## How to uninstall

Remove application with command.

```console
helm uninstall vmalert -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](../../REQUIREMENTS.md).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-alert

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-alert/values.yaml`` file.

<table>
  <thead>
    <th>Key</th>
    <th>Type</th>
    <th>Default</th>
    <th>Description</th>
  </thead>
  <tbody>
    <tr>
      <td>alertmanager.baseURL</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>external URL, that alertmanager will expose to receivers</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.baseURLPrefix</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>external URL Prefix, Prefix for the internal routes of web endpoints</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.config.global.resolve_timeout</td>
      <td>string</td>
      <td><pre lang="">
5m
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.receivers[0].name</td>
      <td>string</td>
      <td><pre lang="">
devnull
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.route.group_by[0]</td>
      <td>string</td>
      <td><pre lang="">
alertname
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.route.group_interval</td>
      <td>string</td>
      <td><pre lang="">
10s
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.route.group_wait</td>
      <td>string</td>
      <td><pre lang="">
30s
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.route.receiver</td>
      <td>string</td>
      <td><pre lang="">
devnull
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.config.route.repeat_interval</td>
      <td>string</td>
      <td><pre lang="">
24h
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>use existing configmap if specified otherwise .config values will be used</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.emptyDir</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.envFrom</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.extraArgs</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.extraContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.image</td>
      <td>object</td>
      <td><pre lang="plaintext">
registry: ""
repository: prom/alertmanager
tag: v0.25.0
</pre>
</td>
      <td><p>alertmanager image configuration</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.imagePullSecrets</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.ingress.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.ingress.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.ingress.tls</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional initContainers to initialize the pod</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.listenAddress</td>
      <td>string</td>
      <td><pre lang="">
0.0.0.0:9093
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.accessModes</td>
      <td>list</td>
      <td><pre lang="plaintext">
- ReadWriteOnce
</pre>
</td>
      <td><p>Array of access modes. Must match those of existing PV or dynamic provisioner. Details are <a href="http://kubernetes.io/docs/user-guide/persistent-volumes/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Persistant volume annotations</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Create/use Persistent Volume Claim for alertmanager component. Empty dir if false</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.existingClaim</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Existing Claim name. If defined, PVC must be created manually before volume will be bound</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.mountPath</td>
      <td>string</td>
      <td><pre lang="">
/data
</pre>
</td>
      <td><p>Mount path. Alertmanager data Persistent Volume mount root path.</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.size</td>
      <td>string</td>
      <td><pre lang="">
50Mi
</pre>
</td>
      <td><p>Size of the volume. Better to set the same as resource limit memory property.</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.storageClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>StorageClass to use for persistent volume. Requires alertmanager.persistentVolume.enabled: true. If defined, PVC created automatically</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.persistentVolume.subPath</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Mount subpath</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.podMetadata.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.podMetadata.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
httpGet:
    path: '{{ ternary "" .baseURLPrefix (empty .baseURLPrefix) }}/-/healthy'
    port: web
</pre>
</td>
      <td><p>liveness probe</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
httpGet:
    path: '{{ ternary "" .baseURLPrefix (empty .baseURLPrefix) }}/-/ready'
    port: web
</pre>
</td>
      <td><p>readiness probe</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
httpGet:
    path: '{{ ternary "" .baseURLPrefix (empty .baseURLPrefix) }}/-/ready'
    port: web
</pre>
</td>
      <td><p>startup probe</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.retention</td>
      <td>string</td>
      <td><pre lang="">
120h
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.securityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Ref: <a href="https://kubernetes.io/docs/user-guide/services/#external-ips" target="_blank">https://kubernetes.io/docs/user-guide/services/#external-ips</a></p>
</td>
    </tr>
    <tr>
      <td>alertmanager.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Ref: <a href="https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip" target="_blank">https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip</a></p>
</td>
    </tr>
    <tr>
      <td>alertmanager.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.ipFamilies</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.nodePort</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>if you want to force a specific nodePort. Must be use with service.type=NodePort</p>
</td>
    </tr>
    <tr>
      <td>alertmanager.service.port</td>
      <td>int</td>
      <td><pre lang="">
9093
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8880
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.templates</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>alertmanager.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
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
      <td></td>
    </tr>
    <tr>
      <td>server.affinity</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Annotations to be added to the deployment</p>
</td>
    </tr>
    <tr>
      <td>server.config.alerts.groups</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>vmalert alert rules configuration configuration: use existing configmap if specified otherwise .config values will be used</p>
</td>
    </tr>
    <tr>
      <td>server.datasource</td>
      <td>object</td>
      <td><pre lang="plaintext">
basicAuth:
    password: ""
    username: ""
bearer:
    token: ""
    tokenFile: ""
url: ""
</pre>
</td>
      <td><p>vmalert reads metrics from source, next section represents its configuration. It can be any service which supports MetricsQL or PromQL.</p>
</td>
    </tr>
    <tr>
      <td>server.datasource.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
password: ""
username: ""
</pre>
</td>
      <td><p>Basic auth for datasource</p>
</td>
    </tr>
    <tr>
      <td>server.datasource.bearer.token</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token with Bearer token. You can use one of token or tokenFile. You don&rsquo;t need to add &ldquo;Bearer&rdquo; prefix string</p>
</td>
    </tr>
    <tr>
      <td>server.datasource.bearer.tokenFile</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token Auth file with Bearer token. You can use one of token or tokenFile</p>
</td>
    </tr>
    <tr>
      <td>server.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
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
      <td><p>Additional containers to run in the same pod</p>
</td>
    </tr>
    <tr>
      <td>server.extraHostPathMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>server.extraVolumeMounts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>server.extraVolumes</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>server.fullnameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.image</td>
      <td>object</td>
      <td><pre lang="plaintext">
pullPolicy: IfNotPresent
registry: ""
repository: victoriametrics/vmalert
tag: ""
variant: ""
</pre>
</td>
      <td><p>vmalert image configuration</p>
</td>
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
      <td></td>
    </tr>
    <tr>
      <td>server.ingress.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.ingress.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.ingress.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
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
      <td></td>
    </tr>
    <tr>
      <td>server.initContainers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional initContainers to initialize the pod</p>
</td>
    </tr>
    <tr>
      <td>server.labels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>labels to be added to the deployment</p>
</td>
    </tr>
    <tr>
      <td>server.minReadySeconds</td>
      <td>int</td>
      <td><pre lang="">
0
</pre>
</td>
      <td><p>specifies the minimum number of seconds for which a newly created Pod should be ready without any of its containers crashing/terminating 0 is the standard k8s default</p>
</td>
    </tr>
    <tr>
      <td>server.name</td>
      <td>string</td>
      <td><pre lang="">
server
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.nameOverride</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.nodeSelector</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.notifier</td>
      <td>object</td>
      <td><pre lang="plaintext">
alertmanager:
    basicAuth:
        password: ""
        username: ""
    bearer:
        token: ""
        tokenFile: ""
    url: ""
</pre>
</td>
      <td><p>Notifier to use for alerts. Multiple notifiers can be enabled by using <code>notifiers</code> section</p>
</td>
    </tr>
    <tr>
      <td>server.notifier.alertmanager.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
password: ""
username: ""
</pre>
</td>
      <td><p>Basic auth for alertmanager</p>
</td>
    </tr>
    <tr>
      <td>server.notifier.alertmanager.bearer.token</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token with Bearer token. You can use one of token or tokenFile. You don&rsquo;t need to add &ldquo;Bearer&rdquo; prefix string</p>
</td>
    </tr>
    <tr>
      <td>server.notifier.alertmanager.bearer.tokenFile</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token Auth file with Bearer token. You can use one of token or tokenFile</p>
</td>
    </tr>
    <tr>
      <td>server.notifiers</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Additional notifiers to use for alerts</p>
</td>
    </tr>
    <tr>
      <td>server.podAnnotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>Annotations to be added to pod</p>
</td>
    </tr>
    <tr>
      <td>server.podDisruptionBudget</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
labels: {}
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Or check <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">docs</a></p>
</td>
    </tr>
    <tr>
      <td>server.podLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.podSecurityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.priorityClassName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.probe.liveness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
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
      <td>server.probe.readiness</td>
      <td>object</td>
      <td><pre lang="plaintext">
failureThreshold: 3
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</pre>
</td>
      <td><p>readiness probe</p>
</td>
    </tr>
    <tr>
      <td>server.probe.startup</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>startup probe</p>
</td>
    </tr>
    <tr>
      <td>server.remote.read.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
password: ""
username: ""
</pre>
</td>
      <td><p>Basic auth for remote read</p>
</td>
    </tr>
    <tr>
      <td>server.remote.read.bearer.token</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token with Bearer token. You can use one of token or tokenFile. You don&rsquo;t need to add &ldquo;Bearer&rdquo; prefix string</p>
</td>
    </tr>
    <tr>
      <td>server.remote.read.bearer.tokenFile</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token Auth file with Bearer token. You can use one of token or tokenFile</p>
</td>
    </tr>
    <tr>
      <td>server.remote.read.url</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.remote.write.basicAuth</td>
      <td>object</td>
      <td><pre lang="plaintext">
password: ""
username: ""
</pre>
</td>
      <td><p>Basic auth for remote write</p>
</td>
    </tr>
    <tr>
      <td>server.remote.write.bearer</td>
      <td>object</td>
      <td><pre lang="plaintext">
token: ""
tokenFile: ""
</pre>
</td>
      <td><p>Auth based on Bearer token for remote write</p>
</td>
    </tr>
    <tr>
      <td>server.remote.write.bearer.token</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token with Bearer token. You can use one of token or tokenFile. You don&rsquo;t need to add &ldquo;Bearer&rdquo; prefix string</p>
</td>
    </tr>
    <tr>
      <td>server.remote.write.bearer.tokenFile</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Token Auth file with Bearer token. You can use one of token or tokenFile</p>
</td>
    </tr>
    <tr>
      <td>server.remote.write.url</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.replicaCount</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.securityContext.enabled</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.clusterIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.externalIPs</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
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
      <td></td>
    </tr>
    <tr>
      <td>server.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.servicePort</td>
      <td>int</td>
      <td><pre lang="">
8880
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.service.type</td>
      <td>string</td>
      <td><pre lang="">
ClusterIP
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.strategy</td>
      <td>object</td>
      <td><pre lang="plaintext">
rollingUpdate:
    maxSurge: 25%
    maxUnavailable: 25%
type: RollingUpdate
</pre>
</td>
      <td><p>deployment strategy, set to standard k8s default</p>
</td>
    </tr>
    <tr>
      <td>server.tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>server.verticalPodAutoscaler</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
</pre>
</td>
      <td><p>Vertical Pod Autoscaler</p>
</td>
    </tr>
    <tr>
      <td>server.verticalPodAutoscaler.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Use VPA for vmalert</p>
</td>
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
      <td>serviceAccount.automountToken</td>
      <td>bool</td>
      <td><pre lang="">
true
</pre>
</td>
      <td><p>mount API token to pod directly</p>
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
  </tbody>
</table>

