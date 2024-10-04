 ![Version: 0.6.5](https://img.shields.io/badge/Version-0.6.5-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-logs-single)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Logs Single version - high-performance, cost-effective and scalable logs storage

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](https://docs.victoriametrics.com/helm/requirements/).

* PV support on underlying infrastructure.

## Chart Details

This chart will do the following:

* Rollout Victoria Logs Single.
* (optional) Rollout [fluentbit](https://fluentbit.io/) to collect logs from pods.

Chart allows to configure logs collection from Kubernetes pods to VictoriaLogs.
In order to do that you need to enable fluentbit:
```yaml
fluent-bit:
  enabled: true
```
By default, fluentbit will forward logs to VictoriaLogs installation deployed by this chart.

## How to install

Access a Kubernetes cluster.

### Setup chart repository (can be omitted for OCI repositories)

Add a chart helm repository with follow commands:

```console
helm repo add vm https://victoriametrics.github.io/helm-charts/

helm repo update
```
List versions of `vm/victoria-logs-single` chart available to installation:

```console
helm search repo vm/victoria-logs-single -l
```

### Install `victoria-logs-single` chart

Export default values of `victoria-logs-single` chart to file `values.yaml`:

  - For HTTPS repository

    ```console
    helm show values vm/victoria-logs-single > values.yaml
    ```
  - For OCI repository

    ```console
    helm show values oci://ghcr.io/victoriametrics/helm-charts/victoria-logs-single > values.yaml
    ```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

  - For HTTPS repository

    ```console
    helm install vls vm/victoria-logs-single -f values.yaml -n NAMESPACE --debug --dry-run
    ```

  - For OCI repository

    ```console
    helm install vls oci://ghcr.io/victoriametrics/helm-charts/victoria-logs-single -f values.yaml -n NAMESPACE --debug --dry-run
    ```

Install chart with command:

  - For HTTPS repository

    ```console
    helm install vls vm/victoria-logs-single -f values.yaml -n NAMESPACE
    ```

  - For OCI repository

    ```console
    helm install vls oci://ghcr.io/victoriametrics/helm-charts/victoria-logs-single -f values.yaml -n NAMESPACE
    ```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'vls'
```

Get the application by running this command:

```console
helm list -f vls -n NAMESPACE
```

See the history of versions of `vls` application with command.

```console
helm history vls -n NAMESPACE
```

## How to uninstall

Remove application with command.

```console
helm uninstall vls -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](https://docs.victoriametrics.com/helm/requirements/).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-logs-single

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-logs-single/values.yaml`` file.

<table class="helm-vars">
  <thead>
    <th class="helm-vars-key">Key</th>
    <th class="helm-vars-type">Type</th>
    <th class="helm-vars-default">Default</th>
    <th class="helm-vars-description">Description</th>
  </thead>
  <tbody>
    <tr>
      <td>extraObjects</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Add extra specs dynamically to this chart</p>
</td>
    </tr>
    <tr>
      <td>fluent-bit</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">config:
    filters: |
        [FILTER]
            Name                kubernetes
            Match               kube.*
            Merge_Log           On
            Keep_Log            On
            K8S-Logging.Parser  On
            K8S-Logging.Exclude On
        [FILTER]
            Name         nest
            Match        *
            Wildcard     pod_name
            Operation    lift
            Nested_under kubernetes
            Add_prefix   kubernetes_
    outputs: |
        [OUTPUT]
            Name             http
            Match            kube.*
            Host             {{ include "victoria-logs.server.fullname" . }}
            port             9428
            compress         gzip
            uri              /insert/jsonline
            format           json_lines
            json_date_format iso8601
            header           AccountID 0
            header           ProjectID 0
            header           VL-Msg-Field log
            header           VL-Time-Field date
            header           VL-Stream-Fields stream,kubernetes_pod_name,kubernetes_container_name,kubernetes_namespace_name
daemonSetVolumeMounts:
    - mountPath: /var/log
      name: varlog
    - mountPath: /var/lib/docker/containers
      name: varlibdockercontainers
      readOnly: true
daemonSetVolumes:
    - hostPath:
        path: /var/log
      name: varlog
    - hostPath:
        path: /var/lib/docker/containers
      name: varlibdockercontainers
enabled: false
resources: {}
</code>
</pre>
</td>
      <td><p>Values for <a href="https://fluent.github.io/helm-charts/" target="_blank">fluent-bit helm chart</a></p>
</td>
    </tr>
    <tr>
      <td>fluent-bit.config.filters</td>
      <td>tpl</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="tpl">
<code class="language-yaml">fluent-bit.config.filters: |
  [FILTER]
      Name                kubernetes
      Match               kube.*
      Merge_Log           On
      Keep_Log            On
      K8S-Logging.Parser  On
      K8S-Logging.Exclude On
  [FILTER]
      Name         nest
      Match        *
      Wildcard     pod_name
      Operation    lift
      Nested_under kubernetes
      Add_prefix   kubernetes_
 
</code>
</pre>
</td>
      <td><p>FluentBit configuration filters</p>
</td>
    </tr>
    <tr>
      <td>fluent-bit.config.outputs</td>
      <td>tpl</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="tpl">
<code class="language-yaml">fluent-bit.config.outputs: |
  [OUTPUT]
      Name             http
      Match            kube.*
      Host             {{ include "victoria-logs.server.fullname" . }}
      port             9428
      compress         gzip
      uri              /insert/jsonline
      format           json_lines
      json_date_format iso8601
      header           AccountID 0
      header           ProjectID 0
      header           VL-Msg-Field log
      header           VL-Time-Field date
      header           VL-Stream-Fields stream,kubernetes_pod_name,kubernetes_container_name,kubernetes_namespace_name
 
</code>
</pre>
</td>
      <td><p>Note that Host must be replaced to match your VictoriaLogs service name Default format points to VictoriaLogs service.</p>
</td>
    </tr>
    <tr>
      <td>fluent-bit.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">false
</code>
</pre>
</td>
      <td><p>Enable deployment of fluent-bit</p>
</td>
    </tr>
    <tr>
      <td>global.compatibility</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">openshift:
    adaptSecurityContext: auto
</code>
</pre>
</td>
      <td><p>Openshift security context compatibility configuration</p>
</td>
    </tr>
    <tr>
      <td>global.image.registry</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Image registry, that can be shared across multiple helm charts</p>
</td>
    </tr>
    <tr>
      <td>global.imagePullSecrets</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Image pull secrets, that can be shared across multiple helm charts</p>
</td>
    </tr>
    <tr>
      <td>global.nameOverride</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Global name override</p>
</td>
    </tr>
    <tr>
      <td>global.victoriaLogs.server.fullnameOverride</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">null
</code>
</pre>
</td>
      <td><p>Overrides the full name of server component</p>
</td>
    </tr>
    <tr>
      <td>global.victoriaLogs.server.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">server
</code>
</pre>
</td>
      <td><p>Server container name</p>
</td>
    </tr>
    <tr>
      <td>podDisruptionBudget</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">enabled: false
extraLabels: {}
</code>
</pre>
</td>
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Details are <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>podDisruptionBudget.extraLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>PodDisruptionBudget extra labels</p>
</td>
    </tr>
    <tr>
      <td>printNotes</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Print chart notes</p>
</td>
    </tr>
    <tr>
      <td>server.affinity</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Pod affinity</p>
</td>
    </tr>
    <tr>
      <td>server.containerWorkingDir</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Container workdir</p>
</td>
    </tr>
    <tr>
      <td>server.emptyDir</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Use an alternate scheduler, e.g. &ldquo;stork&rdquo;. Check details <a href="https://kubernetes.io/docs/tasks/administer-cluster/configure-multiple-schedulers/" target="_blank">here</a>  schedulerName:</p>
</td>
    </tr>
    <tr>
      <td>server.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Enable deployment of server component. Deployed as StatefulSet</p>
</td>
    </tr>
    <tr>
      <td>server.env</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Additional environment variables (ex.: secret tokens, flags). Details are <a href="https://github.com/VictoriaMetrics/VictoriaMetrics#environment-variables" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.envFrom</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Specify alternative source for env variables</p>
</td>
    </tr>
    <tr>
      <td>server.extraArgs</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">envflag.enable: "true"
envflag.prefix: VM_
loggerFormat: json
</code>
</pre>
</td>
      <td><p>Extra command line arguments for container of component</p>
</td>
    </tr>
    <tr>
      <td>server.extraContainers</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Extra containers to run in a pod with Victoria Logs container</p>
</td>
    </tr>
    <tr>
      <td>server.extraHostPathMounts</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Additional hostPath mounts</p>
</td>
    </tr>
    <tr>
      <td>server.extraLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>StatefulSet/Deployment additional labels</p>
</td>
    </tr>
    <tr>
      <td>server.extraVolumeMounts</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Extra Volume Mounts for the container</p>
</td>
    </tr>
    <tr>
      <td>server.extraVolumes</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Extra Volumes for the pod</p>
</td>
    </tr>
    <tr>
      <td>server.image.pullPolicy</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">IfNotPresent
</code>
</pre>
</td>
      <td><p>Image pull policy</p>
</td>
    </tr>
    <tr>
      <td>server.image.registry</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Image registry</p>
</td>
    </tr>
    <tr>
      <td>server.image.repository</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">victoriametrics/victoria-logs
</code>
</pre>
</td>
      <td><p>Image repository</p>
</td>
    </tr>
    <tr>
      <td>server.image.tag</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Image tag</p>
</td>
    </tr>
    <tr>
      <td>server.image.variant</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">victorialogs
</code>
</pre>
</td>
      <td><p>Image tag suffix, which is appended to <code>Chart.AppVersion</code> if no <code>server.image.tag</code> is defined</p>
</td>
    </tr>
    <tr>
      <td>server.imagePullSecrets</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Image pull secrets</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.annotations</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">null
</code>
</pre>
</td>
      <td><p>Ingress annotations</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">false
</code>
</pre>
</td>
      <td><p>Enable deployment of ingress for server component</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.extraLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Ingress extra labels</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.hosts</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Array of host objects</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.ingressClassName</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Ingress controller class name</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.pathType</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">Prefix
</code>
</pre>
</td>
      <td><p>Ingress path type</p>
</td>
    </tr>
    <tr>
      <td>server.ingress.tls</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Array of TLS objects</p>
</td>
    </tr>
    <tr>
      <td>server.initContainers</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Init containers for Victoria Logs Pod</p>
</td>
    </tr>
    <tr>
      <td>server.nodeSelector</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s node selector. Details are <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.accessModes</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">- ReadWriteOnce
</code>
</pre>
</td>
      <td><p>Array of access modes. Must match those of existing PV or dynamic provisioner. Details are <a href="http://kubernetes.io/docs/user-guide/persistent-volumes/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.annotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Persistant volume annotations</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">false
</code>
</pre>
</td>
      <td><p>Create/use Persistent Volume Claim for server component. Empty dir if false</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.existingClaim</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Existing Claim name. If defined, PVC must be created manually before volume will be bound</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.matchLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Bind Persistent Volume by labels. Must match all labels of targeted PV.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.mountPath</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">/storage
</code>
</pre>
</td>
      <td><p>Mount path. Server data Persistent Volume mount root path.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.size</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">3Gi
</code>
</pre>
</td>
      <td><p>Size of the volume. Should be calculated based on the logs you send and retention policy you set.</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.storageClassName</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>StorageClass to use for persistent volume. Requires server.persistentVolume.enabled: true. If defined, PVC created automatically</p>
</td>
    </tr>
    <tr>
      <td>server.persistentVolume.subPath</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Mount subpath</p>
</td>
    </tr>
    <tr>
      <td>server.podAnnotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s annotations</p>
</td>
    </tr>
    <tr>
      <td>server.podLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s additional labels</p>
</td>
    </tr>
    <tr>
      <td>server.podManagementPolicy</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">OrderedReady
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s management policy</p>
</td>
    </tr>
    <tr>
      <td>server.podSecurityContext</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">enabled: true
fsGroup: 2000
runAsNonRoot: true
runAsUser: 1000
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s security context. Details are <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.priorityClassName</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Name of Priority Class</p>
</td>
    </tr>
    <tr>
      <td>server.probe.liveness</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">failureThreshold: 10
initialDelaySeconds: 30
periodSeconds: 30
tcpSocket: {}
timeoutSeconds: 5
</code>
</pre>
</td>
      <td><p>Indicates whether the Container is running. If the liveness probe fails, the kubelet kills the Container, and the Container is subjected to its restart policy. If a Container does not provide a liveness probe, the default state is Success.</p>
</td>
    </tr>
    <tr>
      <td>server.probe.readiness</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">failureThreshold: 3
httpGet: {}
initialDelaySeconds: 5
periodSeconds: 15
timeoutSeconds: 5
</code>
</pre>
</td>
      <td><p>Indicates whether the Container is ready to service requests. If the readiness probe fails, the endpoints controller removes the Pod&rsquo;s IP address from the endpoints of all Services that match the Pod. The default state of readiness before the initial delay is Failure. If a Container does not provide a readiness probe, the default state is Success.</p>
</td>
    </tr>
    <tr>
      <td>server.probe.startup</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Indicates whether the Container is done with potentially costly initialization. If set it is executed first. If it fails Container is restarted. If it succeeds liveness and readiness probes takes over.</p>
</td>
    </tr>
    <tr>
      <td>server.replicaCount</td>
      <td>int</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">1
</code>
</pre>
</td>
      <td><p>Replica count</p>
</td>
    </tr>
    <tr>
      <td>server.resources</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Resource object. Details are <a href="http://kubernetes.io/docs/user-guide/compute-resources/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.retentionPeriod</td>
      <td>int</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">1
</code>
</pre>
</td>
      <td><p>Data retention period in month</p>
</td>
    </tr>
    <tr>
      <td>server.securityContext</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">allowPrivilegeEscalation: false
capabilities:
    drop:
        - ALL
enabled: true
readOnlyRootFilesystem: true
</code>
</pre>
</td>
      <td><p>Security context to be added to server pods</p>
</td>
    </tr>
    <tr>
      <td>server.service.annotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Service annotations</p>
</td>
    </tr>
    <tr>
      <td>server.service.clusterIP</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Service ClusterIP</p>
</td>
    </tr>
    <tr>
      <td>server.service.externalIPs</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Service external IPs. Details are <a href="https://kubernetes.io/docs/user-guide/services/#external-ips" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.service.externalTrafficPolicy</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Service external traffic policy. Check <a href="https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip" target="_blank">here</a> for details</p>
</td>
    </tr>
    <tr>
      <td>server.service.healthCheckNodePort</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Health check node port for a service. Check <a href="https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip" target="_blank">here</a> for details</p>
</td>
    </tr>
    <tr>
      <td>server.service.ipFamilies</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>List of service IP families. Check <a href="https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services" target="_blank">here</a> for details.</p>
</td>
    </tr>
    <tr>
      <td>server.service.ipFamilyPolicy</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Service IP family policy. Check <a href="https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services" target="_blank">here</a> for details.</p>
</td>
    </tr>
    <tr>
      <td>server.service.labels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Service labels</p>
</td>
    </tr>
    <tr>
      <td>server.service.loadBalancerIP</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Service load balacner IP</p>
</td>
    </tr>
    <tr>
      <td>server.service.loadBalancerSourceRanges</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Load balancer source range</p>
</td>
    </tr>
    <tr>
      <td>server.service.servicePort</td>
      <td>int</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">9428
</code>
</pre>
</td>
      <td><p>Service port</p>
</td>
    </tr>
    <tr>
      <td>server.service.type</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">ClusterIP
</code>
</pre>
</td>
      <td><p>Service type</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.annotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Service Monitor annotations</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.basicAuth</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Basic auth params for Service Monitor</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">false
</code>
</pre>
</td>
      <td><p>Enable deployment of Service Monitor for server component. This is Prometheus operator object</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.extraLabels</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Service Monitor labels</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.metricRelabelings</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Service Monitor metricRelabelings</p>
</td>
    </tr>
    <tr>
      <td>server.serviceMonitor.relabelings</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Service Monitor relabelings</p>
</td>
    </tr>
    <tr>
      <td>server.statefulSet.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Creates statefulset instead of deployment, useful when you want to keep the cache</p>
</td>
    </tr>
    <tr>
      <td>server.statefulSet.podManagementPolicy</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">OrderedReady
</code>
</pre>
</td>
      <td><p>Deploy order policy for StatefulSet pods</p>
</td>
    </tr>
    <tr>
      <td>server.terminationGracePeriodSeconds</td>
      <td>int</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">60
</code>
</pre>
</td>
      <td><p>Pod&rsquo;s termination grace period in seconds</p>
</td>
    </tr>
    <tr>
      <td>server.tolerations</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Node tolerations for server scheduling to nodes with taints. Details are <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>server.topologySpreadConstraints</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">[]
</code>
</pre>
</td>
      <td><p>Pod topologySpreadConstraints</p>
</td>
    </tr>
  </tbody>
</table>

