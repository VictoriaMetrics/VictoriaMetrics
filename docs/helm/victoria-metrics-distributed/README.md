![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.4.0](https://img.shields.io/badge/Version-0.4.0-informational?style=flat-square)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/helm/victoriametrics/victoria-metrics-distributed)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

A Helm chart for Running VMCluster on Multiple Availability Zones

## Prerequisites

* Install the follow packages: ``git``, ``kubectl``, ``helm``, ``helm-docs``. See this [tutorial](https://docs.victoriametrics.com/helm/requirements/).

* PV support on underlying infrastructure.

* Multiple availability zones.

## Chart Details

This chart sets up multiple VictoriaMetrics cluster instances on multiple [availability zones](https://kubernetes.io/docs/setup/best-practices/multiple-zones/), provides both global write and read entrypoints.

The default setup is as shown below:

![victoriametrics-distributed-topology](./img/victoriametrics-distributed-topology.webp)

For write:
1. extra-vmagent(optional): scrapes external targets and all the components installed by this chart, sends data to global write entrypoint.
2. vmauth-global-write: global write entrypoint, proxies requests to one of the zone `vmagent` with `least_loaded` policy.
3. vmagent(per-zone): remote writes data to availability zones that enabled `.Values.availabilityZones.allowIngest`, and [buffer data on disk](https://docs.victoriametrics.com/vmagent/#calculating-disk-space-for-persistence-queue) when zone is unavailable to ingest.
4. vmauth-write-balancer(per-zone): proxies requests to vminsert instances inside it's zone with `least_loaded` policy.
5. vmcluster(per-zone): processes write requests and stores data.

For read:
1. vmcluster(per-zone): processes query requests and returns results.
2. vmauth-read-balancer(per-zone): proxies requests to vmselect instances inside it's zone with `least_loaded` policy.
3. vmauth-read-proxy(per-zone): uses all the `vmauth-read-balancer` as servers if zone has `.Values.availabilityZones.allowQuery` enabled, always prefer "local" `vmauth-read-balancer` to reduce cross-zone traffic with `first_available` policy.
4. vmauth-global-read: global query entrypoint, proxies requests to one of the zone `vnauth-read-proxy` with `first_available` policy.
5. grafana(optional): uses `vmauth-global-read` as default datasource.

>Note:
As the topology shown above, this chart doesn't include components like vmalert, alertmanager, etc by default.
You can install them using dependency [victoria-metrics-k8s-stack](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-k8s-stack) or having separate release.

### Why use `victoria-metrics-distributed` chart?

One of the best practice of running production kubernetes cluster is running with [multiple availability zones](https://kubernetes.io/docs/setup/best-practices/multiple-zones/). And apart from kubernetes control plane components, we also want to spread our application pods on multiple zones, to continue serving even if zone outage happens.

VictoriaMetrics supports [data replication](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety) natively which can guarantees data availability when part of the vmstorage instances failed. But it doesn't works well if vmstorage instances are spread on multiple availability zones, since data replication could be stored on single availability zone, which will be lost when zone outage happens.
To avoid this, vmcluster must be installed on multiple availability zones, each containing a 100% copy of data. As long as one zone is available, both global write and read entrypoints should work without interruption.

### How to write data?

The chart provides `vmauth-global-write` as global write entrypoint, it supports [push-based data ingestion protocols](https://docs.victoriametrics.com/vmagent/#how-to-push-data-to-vmagent) as VictoriaMetrics does.
Optionally, you can push data to any of the per-zone vmagents, and they will replicate the received data across zones.

### How to query data?

The chart provides `vmauth-global-read` as global read entrypoint, it picks the first available zone (see [first_available](https://docs.victoriametrics.com/vmauth/#high-availability) policy) as it's preferred datasource and switches automatically to next zone if first one is unavailable, check [vmauth `first_available`](https://docs.victoriametrics.com/vmauth/#high-availability) for more details.
If you have services like [vmalert](https://docs.victoriametrics.com/vmalert) or Grafana deployed in each zone, then configure them to use local `vmauth-read-proxy`. Per-zone `vmauth-read-proxy` always prefers "local" vmcluster for querying and reduces cross-zone traffic. 

You can also pick other proxies like kubernetes service which supports [Topology Aware Routing](https://kubernetes.io/docs/concepts/services-networking/topology-aware-routing/) as global read entrypoint.

### What happens if zone outage happen?

If availability zone `zone-eu-1` is experiencing an outage, `vmauth-global-write` and `vmauth-global-read` will work without interruption:
1. `vmauth-global-write` stops proxying write requests to `zone-eu-1` automatically;
2. `vmauth-global-read` and `vmauth-read-proxy` stops proxying read requests to `zone-eu-1` automatically;
3. `vmagent` on `zone-us-1` fails to send data to `zone-eu-1.vmauth-write-balancer`, starts to buffer data on disk(unless `-remoteWrite.disableOnDiskQueue` is specified, which is not recommended for this topology);
To keep data completeness for all the availability zones, make sure you have enough disk space on vmagent for buffer, see [this doc](https://docs.victoriametrics.com/vmagent/#calculating-disk-space-for-persistence-queue) for size recommendation.

And to avoid getting incomplete responses from `zone-eu-1` which gets recovered from outage, check vmagent on `zone-us-1` to see if persistent queue has been drained. If not, remove `zone-eu-1` from serving query by setting `.Values.availabilityZones.{zone-eu-1}.allowQuery=false` and change it back after confirm all data are restored.

### How to use [multitenancy](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy)?

By default, all the data that written to `vmauth-global-write` belong to tenant `0`. To write data to different tenants, set `.Values.enableMultitenancy=true` and create new tenant users for `vmauth-global-write`.
For example, writing data to tenant `1088` with following steps:
1. create tenant VMUser for vmauth `vmauth-global-write` to use:
```
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMUser
metadata:
  name: tenant-1088-rw
  labels:
    tenant-test: "true"
spec:
  targetRefs:
  - static:
      ## list all the zone vmagent here
      url: "http://vmagent-vmagent-zone-eu-1:8429"
      url: "http://vmagent-vmagent-zone-us-1:8429"
    paths:
    - "/api/v1/write"
    - "/prometheus/api/v1/write"
    - "/write"
    - "/api/v1/import"
    - "/api/v1/import/.+"
    target_path_suffix: /insert/1088/
  username: tenant-1088
  password: secret
```

Add extra VMUser selector in vmauth `vmauth-global-write`
```
spec:
  userSelector:
    matchLabels:
      tenant-test: "true"
```

2. send data to `vmauth-global-write` using above token.
Example command using vmagent:
```
/path/to/vmagent -remoteWrite.url=http://vmauth-vmauth-global-write-$ReleaseName-vm-distributed:8427/prometheus/api/v1/write -remoteWrite.basicAuth.username=tenant-1088 -remoteWrite.basicAuth.password=secret
```

## How to install

Access a Kubernetes cluster.

### Setup chart repository (can be omitted for OCI repositories)

Add a chart helm repository with follow commands:

```console
helm repo add vm https://victoriametrics.github.io/helm-charts/

helm repo update
```
List versions of `vm/victoria-metrics-distributed` chart available to installation:

```console
helm search repo vm/victoria-metrics-distributed -l
```

### Install `victoria-metrics-distributed` chart

Export default values of `victoria-metrics-distributed` chart to file `values.yaml`:

  - For HTTPS repository

    ```console
    helm show values vm/victoria-metrics-distributed > values.yaml
    ```
  - For OCI repository

    ```console
    helm show values oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-distributed > values.yaml
    ```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

  - For HTTPS repository

    ```console
    helm install vmd vm/victoria-metrics-distributed -f values.yaml -n NAMESPACE --debug --dry-run
    ```

  - For OCI repository

    ```console
    helm install vmd oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-distributed -f values.yaml -n NAMESPACE --debug --dry-run
    ```

Install chart with command:

  - For HTTPS repository

    ```console
    helm install vmd vm/victoria-metrics-distributed -f values.yaml -n NAMESPACE
    ```

  - For OCI repository

    ```console
    helm install vmd oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-distributed -f values.yaml -n NAMESPACE
    ```

Get the pods lists by running this commands:

```console
kubectl get pods -A | grep 'vmd'
```

Get the application by running this command:

```console
helm list -f vmd -n NAMESPACE
```

See the history of versions of `vmd` application with command.

```console
helm history vmd -n NAMESPACE
```

## How to upgrade

In order to serving query and ingestion while upgrading components version or changing configurations, it's recommended to perform maintenance on availability zone one by one.
First, performing update on availability zone `zone-eu-1`:
1. remove `zone-eu-1` from serving query by setting `.Values.availabilityZones.{zone-eu-1}.allowQuery=false`;
2. run `helm upgrade vm-dis -n NAMESPACE` with updated configurations for `zone-eu-1` in `values.yaml`;
3. wait for all the components on zone `zone-eu-1` running;
4. wait `zone-us-1` vmagent persistent queue for `zone-eu-1` been drained, add `zone-eu-1` back to serving query by setting `.Values.availabilityZones.{zone-eu-1}.allowQuery=true`.

Then, perform update on availability zone `zone-us-1` with the same steps1~4.

## How to uninstall

Remove application with command.

```console
helm uninstall vmd -n NAMESPACE
```

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](https://docs.victoriametrics.com/helm/requirements/).

Generate docs with ``helm-docs`` command.

```bash
cd charts/victoria-metrics-distributed

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.

## Parameters

The following tables lists the configurable parameters of the chart and their default values.

Change the values according to the need of the environment in ``victoria-metrics-distributed`/values.yaml`` file.

<table class="helm-vars">
  <thead>
    <th class="helm-vars-key">Key</th>
    <th class="helm-vars-type">Type</th>
    <th class="helm-vars-default">Default</th>
    <th class="helm-vars-description">Description</th>
  </thead>
  <tbody>
    <tr>
      <td>availabilityZones[0].allowIngest</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Allow data ingestion to this zone</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].allowQuery</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Allow data query from this zone through global query endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].extraAffinity</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Extra affinity adds user defined custom affinity rules</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">zone-eu-1
</code>
</pre>
</td>
      <td><p>Availability zone name</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].nodeSelector</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">topology.kubernetes.io/zone: zone-eu-1
</code>
</pre>
</td>
      <td><p>Node selector to restrict where pods of this zone can be placed. usually provided by cloud providers.</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].topologySpreadConstraints</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
</code>
</pre>
</td>
      <td><p>Topology spread constraints allows to customize the default topologySpreadConstraints.</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmagent</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">annotations: {}
enabled: true
name: ""
spec: {}
</code>
</pre>
</td>
      <td><p>VMAgent here only meant to proxy write requests to each az, doesn&rsquo;t support customized other remote write address.</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmagent.annotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAgent annotations</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmagent.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmagent object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmagent.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAgent spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmagentspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthCrossAZQuery.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create a vmauth with all the zone with <code>allowQuery: true</code> as query backends</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthCrossAZQuery.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthCrossAZQuery.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthIngest.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create vmauth as a local write endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthIngest.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthIngest.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">extraArgs:
    discoverBackendIPs: "true"
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthQueryPerZone.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create vmauth as a local read endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthQueryPerZone.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmauthQueryPerZone.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">extraArgs:
    discoverBackendIPs: "true"
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmcluster.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmcluster.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmcluster, by default is vmcluster-<zoneName></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[0].vmcluster.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">replicationFactor: 2
retentionPeriod: "14"
vminsert:
    extraArgs: {}
    replicaCount: 2
    resources: {}
vmselect:
    extraArgs: {}
    replicaCount: 2
    resources: {}
vmstorage:
    replicaCount: 2
    resources: {}
    storageDataPath: /vm-data
</code>
</pre>
</td>
      <td><p>Spec for VMCluster CRD, see <a href="https://docs.victoriametrics.com/operator/api#vmclusterspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].allowIngest</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Allow data ingestion to this zone</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].allowQuery</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Allow data query from this zone through global query endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].extraAffinity</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>Extra affinity adds user defined custom affinity rules</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">zone-us-1
</code>
</pre>
</td>
      <td><p>Availability zone name</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].nodeSelector</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">topology.kubernetes.io/zone: zone-us-1
</code>
</pre>
</td>
      <td><p>Node selector to restrict where pods of this zone can be placed. usually provided by cloud providers.</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].topologySpreadConstraints</td>
      <td>list</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
</code>
</pre>
</td>
      <td><p>Topology spread constraints allows to customize the default topologySpreadConstraints.</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmagent</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">annotations: {}
enabled: true
name: ""
spec: {}
</code>
</pre>
</td>
      <td><p>VMAgent only meant to proxy write requests to each az, doesn&rsquo;t support customized remote write address</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmagent.annotations</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAgent annotations</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmagent.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmagent object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmagent.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAgent spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmagentspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthCrossAZQuery.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create a vmauth with all the zone with <code>allowQuery: true</code> as query backends</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthCrossAZQuery.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthCrossAZQuery.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthIngest.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create vmauth as a local write endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthIngest.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthIngest.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">extraArgs:
    discoverBackendIPs: "true"
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthQueryPerZone.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create vmauth as a local read endpoint</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthQueryPerZone.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmauthQueryPerZone.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">extraArgs:
    discoverBackendIPs: "true"
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmcluster.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td></td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmcluster.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmcluster, by default is vmcluster-<zoneName></p>
</td>
    </tr>
    <tr>
      <td>availabilityZones[1].vmcluster.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">replicationFactor: 2
retentionPeriod: "14"
vminsert:
    extraArgs: {}
    replicaCount: 2
    resources: {}
vmselect:
    extraArgs: {}
    replicaCount: 2
    resources: {}
vmstorage:
    replicaCount: 2
    resources: {}
    storageDataPath: /vm-data
</code>
</pre>
</td>
      <td><p>Spec for VMCluster CRD, see <a href="https://docs.victoriametrics.com/operator/api#vmclusterspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>enableMultitenancy</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">false
</code>
</pre>
</td>
      <td><p>Enable multitenancy mode see <a href="https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-distributed#how-to-use-multitenancy" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>extraVMAgent</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">enabled: true
name: test-vmagent
spec:
    selectAllByDefault: true
</code>
</pre>
</td>
      <td><p>Set up an extra vmagent to scrape all the scrape objects by default, and write data to above vmauth-global-ingest endpoint.</p>
</td>
    </tr>
    <tr>
      <td>fullnameOverride</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Overrides the chart&rsquo;s computed fullname.</p>
</td>
    </tr>
    <tr>
      <td>nameOverride</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">vm-distributed
</code>
</pre>
</td>
      <td><p>Overrides the chart&rsquo;s name</p>
</td>
    </tr>
    <tr>
      <td>victoria-metrics-k8s-stack</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">alertmanager:
    enabled: false
crds:
    enabled: true
enabled: true
grafana:
    enabled: true
    sidecar:
        datasources:
            enabled: true
victoria-metrics-operator:
    enabled: true
vmagent:
    enabled: false
vmalert:
    enabled: false
vmcluster:
    enabled: false
vmsingle:
    enabled: false
</code>
</pre>
</td>
      <td><p>Set up vm operator and other resources like vmalert, grafana if needed</p>
</td>
    </tr>
    <tr>
      <td>vmauthIngestGlobal.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create a vmauth as the global write entrypoint</p>
</td>
    </tr>
    <tr>
      <td>vmauthIngestGlobal.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>vmauthIngestGlobal.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
    <tr>
      <td>vmauthQueryGlobal.enabled</td>
      <td>bool</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">true
</code>
</pre>
</td>
      <td><p>Create vmauth as the global read entrypoint</p>
</td>
    </tr>
    <tr>
      <td>vmauthQueryGlobal.name</td>
      <td>string</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="">
<code class="language-yaml">""
</code>
</pre>
</td>
      <td><p>Override the name of the vmauth object</p>
</td>
    </tr>
    <tr>
      <td>vmauthQueryGlobal.spec</td>
      <td>object</td>
      <td><pre class="helm-vars-default-value" language-yaml" lang="plaintext">
<code class="language-yaml">{}
</code>
</pre>
</td>
      <td><p>VMAuth spec. More options can be found <a href="https://docs.victoriametrics.com/operator/api/#vmauthspec" target="_blank">here</a></p>
</td>
    </tr>
  </tbody>
</table>

