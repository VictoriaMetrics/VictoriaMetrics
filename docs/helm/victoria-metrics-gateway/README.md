
![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.4.0](https://img.shields.io/badge/Version-0.4.0-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-gateway)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

Victoria Metrics Gateway - Auth & Rate-Limitting proxy for Victoria Metrics

# Table of Content

* [Prerequisites](#prerequisites)
* [Chart Details](#chart-details)
* [How to Install](#how-to-install)
* [How to Uninstall](#how-to-uninstall)
* [How to use JWT signature verification](#how-to-use-jwt-signature-verification)
* [Documentation of Helm Chart](#documentation-of-helm-chart)

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](../../REQUIREMENTS.md).
* PV support on underlying infrastructure

## Chart Details

This chart will do the following:

* Rollout victoria metrics gateway

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

List versions of `vm/victoria-metrics-gateway` chart available to installation:

```console
helm search repo vm/victoria-metrics-gateway -l
```

Export default values of ``victoria-metrics-gateway`` chart to file ``values.yaml``:

```console
helm show values vm/victoria-metrics-gateway > values.yaml
```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

```console
helm install vmgateway vm/victoria-metrics-gateway -f values.yaml -n NAMESPACE --debug --dry-run
```

Install chart with command:

```console
helm install vmgateway vm/victoria-metrics-gateway -f values.yaml -n NAMESPACE
```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'vmgateway'
```

Get the application by running this command:

```console
helm list -f vmgateway -n NAMESPACE
```

See the history of versions of ``vmgateway`` application with command.

```console
helm history vmgateway -n NAMESPACE
```

## How to uninstall

Remove application with command.

```console
helm uninstall vmgateway -n NAMESPACE
```

# How to use [JWT signature verification](https://docs.victoriametrics.com/vmgateway#jwt-signature-verification)

Kubernetes best-practice is to store sensitive configuration parts in secrets. For example, 2 keys will be stored as:
```yaml
apiVersion: v1
data:
  key: "<<KEY_DATA>>"
kind: Secret
metadata:
  name: key1
---
apiVersion: v1
data:
  key: "<<KEY_DATA>>"
kind: Secret
metadata:
  name: key2
```

In order to use those secrets it is needed to:
- mount secrets into pod
- provide flag pointing to secret on disk

Here is an example `values.yml` file configuration to achieve this:
```yaml
auth:
  enable: true

extraVolumes:
  - name: key1
    secret:
      secretName: key1
  - name: key2
    secret:
      secretName: key2

extraVolumeMounts:
  - name: key1
    mountPath: /key1
  - name: key2
    mountPath: /key2

extraArgs:
  envflag.enable: "true"
  envflag.prefix: VM_
  loggerFormat: json
  auth.publicKeyFiles: "/key1/key,/key2/key"
```
Note that in this configuration all secret keys will be mounted and accessible to pod.
Please, refer to [this](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#secretvolumesource-v1-core) doc to see all available secret source options.

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](../../REQUIREMENTS.md).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-gateway

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-gateway/values.yaml`` file.

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
      <td>auth</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: false
</pre>
</td>
      <td><p>Access Control configuration. <a href="https://docs.victoriametrics.com/vmgateway#access-control" target="_blank">https://docs.victoriametrics.com/vmgateway#access-control</a></p>
</td>
    </tr>
    <tr>
      <td>auth.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable/Disable access-control</p>
</td>
    </tr>
    <tr>
      <td>clusterMode</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Specify to True if the source for rate-limiting, reading and writing as a VictoriaMetrics Cluster. Must be true for rate limiting</p>
</td>
    </tr>
    <tr>
      <td>configMap</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Use existing configmap if specified otherwise .config values will be used. Ref: <a href="https://docs.victoriametrics.com/vmgateway" target="_blank">https://docs.victoriametrics.com/vmgateway</a></p>
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
      <td><p>Additional environment variables (ex.: secret tokens, flags) <a href="https://github.com/VictoriaMetrics/VictoriaMetrics#environment-variables" target="_blank">https://github.com/VictoriaMetrics/VictoriaMetrics#environment-variables</a></p>
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
      <td><p>Victoria Metrics gateway Docker registry</p>
</td>
    </tr>
    <tr>
      <td>image.repository</td>
      <td>string</td>
      <td><pre lang="">
victoriametrics/vmgateway
</pre>
</td>
      <td><p>Victoria Metrics gateway Docker repository and image name</p>
</td>
    </tr>
    <tr>
      <td>image.tag</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Tag of Docker image override Chart.AppVersion</p>
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
      <td>rateLimiter</td>
      <td>object</td>
      <td><pre lang="plaintext">
config: {}
datasource:
    url: ""
enabled: false
</pre>
</td>
      <td><p>Rate limiter configuration. Docs <a href="https://docs.victoriametrics.com/vmgateway#rate-limiter" target="_blank">https://docs.victoriametrics.com/vmgateway#rate-limiter</a></p>
</td>
    </tr>
    <tr>
      <td>rateLimiter.datasource.url</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Datasource VictoriaMetrics or vmselects. Required. Example <a href="http://victoroametrics:8428" target="_blank">http://victoroametrics:8428</a> or <a href="http://vmselect:8481/select/0/prometheus" target="_blank">http://vmselect:8481/select/0/prometheus</a></p>
</td>
    </tr>
    <tr>
      <td>rateLimiter.enabled</td>
      <td>bool</td>
      <td><pre lang="">
false
</pre>
</td>
      <td><p>Enable/Disable rate-limiting</p>
</td>
    </tr>
    <tr>
      <td>read.url</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Read endpoint without suffixes, victoriametrics or vmselect. Example <a href="http://victoroametrics:8428" target="_blank">http://victoroametrics:8428</a> or <a href="http://vmselect:8481" target="_blank">http://vmselect:8481</a></p>
</td>
    </tr>
    <tr>
      <td>replicaCount</td>
      <td>int</td>
      <td><pre lang="">
1
</pre>
</td>
      <td><p>Number of replicas of vmgateway</p>
</td>
    </tr>
    <tr>
      <td>resources</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
</pre>
</td>
      <td><p>We usually recommend not to specify default resources and to leave this as a conscious choice for the user. This also increases chances charts run on environments with little resources, such as Minikube. If you do want to specify resources, uncomment the following lines, adjust them as necessary, and remove the curly braces after &lsquo;resources:&rsquo;.</p>
</td>
    </tr>
    <tr>
      <td>securityContext</td>
      <td>object</td>
      <td><pre lang="plaintext">
enabled: true
runAsGroup: 1000
runAsNonRoot: true
runAsUser: 1000
</pre>
</td>
      <td><p>Ref: <a href="https://kubernetes.io/docs/tasks/configure-pod-container/security-context/" target="_blank">https://kubernetes.io/docs/tasks/configure-pod-container/security-context/</a></p>
</td>
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
      <td>service.extraLabels</td>
      <td>object</td>
      <td><pre lang="plaintext">
{}
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
8431
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
    <tr>
      <td>write.url</td>
      <td>string</td>
      <td><pre lang="">
""
</pre>
</td>
      <td><p>Write endpoint without suffixes, victoriametrics or vminsert. Example <a href="http://victoroametrics:8428" target="_blank">http://victoroametrics:8428</a> or <a href="http://vminsert:8480" target="_blank">http://vminsert:8480</a></p>
</td>
    </tr>
  </tbody>
</table>

