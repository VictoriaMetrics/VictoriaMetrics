![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.6.0](https://img.shields.io/badge/Version-0.6.0-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-auth)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Metrics Auth - is a simple auth proxy and router for VictoriaMetrics.

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
List versions of `vm/victoria-metrics-auth` chart available to installation:

```console
helm search repo vm/victoria-metrics-auth -l
```

### Install `victoria-metrics-auth` chart

Export default values of `victoria-metrics-auth` chart to file `values.yaml`:

  - For HTTPS repository

    ```console
    helm show values vm/victoria-metrics-auth > values.yaml
    ```
  - For OCI repository

    ```console
    helm show values oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-auth > values.yaml
    ```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

  - For HTTPS repository

    ```console
    helm install vma vm/victoria-metrics-auth -f values.yaml -n NAMESPACE --debug --dry-run
    ```

  - For OCI repository

    ```console
    helm install vma oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-auth -f values.yaml -n NAMESPACE --debug --dry-run
    ```

Install chart with command:

  - For HTTPS repository

    ```console
    helm install vma vm/victoria-metrics-auth -f values.yaml -n NAMESPACE
    ```

  - For OCI repository

    ```console
    helm install vma oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-auth -f values.yaml -n NAMESPACE
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
cd charts/victoria-metrics-auth

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-auth/values.yaml`` file.

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
      <td><p>Affinity configurations</p>
</td>
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
      <td>config</td>
      <td>string</td>
      <td><pre lang="">
null
</pre>
</td>
      <td><p>Config file content.</p>
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
      <td>extraArgs."envflag.enable"</td>
      <td>string</td>
      <td><pre lang="">
"true"
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>extraArgs."envflag.prefix"</td>
      <td>string</td>
      <td><pre lang="">
VM_
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>extraArgs.loggerFormat</td>
      <td>string</td>
      <td><pre lang="">
json
</pre>
</td>
      <td></td>
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
      <td><p>Labels to be added to the deployment and pods</p>
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
      <td>image.pullPolicy</td>
      <td>string</td>
      <td><pre lang="">
IfNotPresent
</pre>
</td>
      <td><p>Pull policy of Docker image</p>
</td>
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
victoriametrics/vmauth
</pre>
</td>
      <td><p>Victoria Metrics Auth Docker repository and image name</p>
</td>
    </tr>
    <tr>
      <td>image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Tag of Docker image</p>
</td>
    </tr>
    <tr>
      <td>image.variant</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td></td>
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
      <td>ingressInternal.annotations</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingressInternal.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingressInternal.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingressInternal.hosts</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>ingressInternal.pathType</td>
      <td>string</td>
      <td><pre lang="">
Prefix
</pre>
</td>
      <td><p>pathType is only for k8s &gt;= 1.1=</p>
</td>
    </tr>
    <tr>
      <td>ingressInternal.tls</td>
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
      <td><p>NodeSelector configurations. Ref: <a href="https://kubernetes.io/docs/user-guide/node-selection/" target="_blank">https://kubernetes.io/docs/user-guide/node-selection/</a></p>
</td>
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
      <td><p>See <code>kubectl explain poddisruptionbudget.spec</code> for more. Ref: <a href="https://kubernetes.io/docs/tasks/run-application/configure-pdb/" target="_blank">https://kubernetes.io/docs/tasks/run-application/configure-pdb/</a></p>
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
initialDelaySeconds: 5
periodSeconds: 15
tcpSocket: {}
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
      <td>rbac.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>replicaCount</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Number of replicas of vmauth</p>
</td>
    </tr>
    <tr>
      <td>resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>We usually recommend not to specify default resources and to leave this as a conscious choice for the user. This also increases chances charts run on environments with little resources, such as Minikube. If you do want to specify resources, uncomment the following lines, adjust them as necessary, and remove the curly braces after <code>resources:</code>.</p>
</td>
    </tr>
    <tr>
      <td>secretName</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Use existing secret if specified otherwise .config values will be used. Ref: <a href="https://docs.victoriametrics.com/vmauth" target="_blank">https://docs.victoriametrics.com/vmauth</a>. Configuration in the given secret must be stored under <code>auth.yml</code> key.</p>
</td>
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
true
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
8427
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
      <td>tolerations</td>
      <td>list</td>
      <td><pre lang="plaintext">
[]
</pre>
</td>
      <td><p>Tolerations configurations. Ref: <a href="https://kubernetes.io/docs/concepts/configuration/assign-pod-node/" target="_blank">https://kubernetes.io/docs/concepts/configuration/assign-pod-node/</a></p>
</td>
    </tr>
  </tbody>
</table>

