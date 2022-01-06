---
sort: 6
---

# Design

This document describes the design and interaction between the custom resource definitions (CRD) that the Victoria 
Metrics Operator introduces.

Operator introduces the following custom resources:

* [VMSingle](#vmsingle)
* [VMCluster](#vmcluster)
* [VMAgent](#vmagent)
* [VMAlert](#vmalert)
* [VMServiceScrape](#vmservicescrape)
* [VMPodScrape](#vmpodscrape)
* [VMAlertmanager](#vmalertmanager)
* [VMAlertmanagerConfig](#vmalertmanagerconfig)
* [VMRule](#vmrule)
* [VMProbe](#vmprobe)
* [VMNodeScrape](#vmodescrape)
* [VMStaticScrape](#vmstaticscrape)
* [VMAuth](#vmauth)
* [VMUser](#vmuser)

## VMSingle

The `VMSingle` CRD declaratively defines a [single-node VM](https://github.com/VictoriaMetrics/VictoriaMetrics) 
installation to run in a Kubernetes cluster. 

For each `VMSingle` resource, the Operator deploys a properly configured `Deployment` in the same namespace. 
The VMSingle `Pod`s are configured to mount an empty dir or  `PersistentVolumeClaimSpec` for storing data. 
Deployment update strategy set to [recreate](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#recreate-deployment). 
No more than one replica allowed.

For each `VMSingle` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with 
name `<VMSingle-name>`.

## VMCluster
 
The `VMCluster` CRD defines a [cluster version VM](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster). 

For each `VMCluster` resource, the Operator creates `VMStorage` as `StatefulSet`, `VMSelect` as `StatefulSet` and `VMInsert` 
as deployment. For `VMStorage` and `VMSelect` headless  services are created. `VMInsert` is created as service with clusterIP. 

There is a strict order for these objects creation and reconciliation:
 1. `VMStorage` is synced - the Operator waits until all its pods are ready;
 2. Then it syncs `VMSelect` with the same manner;
 3. `VMInsert` is the last object to sync.

All statefulsets are created with [OnDelete](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#on-delete) 
update type. It allows to manually manage the rolling update process for Operator by deleting pods one by one and waiting 
for the ready status.

Rolling update process may be configured by the operator env variables. 
The most important is `VM_PODWAITREADYTIMEOUT=80s` - it controls how long to wait for pod's ready status.

## VMAgent

The `VMAgent` CRD declaratively defines a desired [VMAgent](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmagent) 
setup to run in a Kubernetes cluster. 

For each `VMAgent` resource Operator deploys a properly configured `Deployment` in the same namespace. 
The VMAgent `Pod`s are configured to mount a `Secret` prefixed with `<VMAgent-name>` containing the configuration 
for VMAgent.

For each `VMAgent` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with 
name `<VMAgent-name>`.

The CRD specifies which `VMServiceScrape` should be covered by the deployed VMAgent instances based on label selection. 
The Operator then generates a configuration based on the included `VMServiceScrape`s and updates the `Secret` which 
contains the configuration. It continuously does so for all changes that are made to the `VMServiceScrape`s or the 
`VMAgent` resource itself.

If no selection of `VMServiceScrape`s is provided - Operator leaves management of the `Secret` to the user, 
so user can set custom configuration while still benefiting from the Operator's capabilities of managing VMAgent setups.

## VMAlert

The `VMAlert` CRD declaratively defines a desired [VMAlert](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmalert) 
setup to run in a Kubernetes cluster. 

For each `VMAlert` resource, the Operator deploys a properly configured `Deployment` in the same namespace. 
The VMAlert `Pod`s are configured to mount a list of `Configmaps` prefixed with `<VMAlert-name>-number` containing 
the configuration for alerting rules.

For each `VMAlert` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with 
name `<VMAlert-name>`.

The CRD specifies which `VMRule`s should be covered by the deployed VMAlert instances based on label selection. 
The Operator then generates a configuration based on the included `VMRule`s and updates the `Configmaps` containing 
the configuration. It continuously does so for all changes that are made to `VMRule`s or to the `VMAlert` resource itself.

Alerting rules are filtered by selector `ruleNamespaceSelector` in `VMAlert` CRD definition. For selecting rules from all 
namespaces you must specify it to empty value:

```yaml
spec:
 ruleNamespaceSelector: {}
```


## VMServiceScrape

The `VMServiceScrape` CRD allows to define a dynamic set of services for monitoring. Services 
and scraping configurations can be matched via label selections. This allows an organization to introduce conventions 
for how metrics should be exposed. Following these conventions new services will be discovered automatically without 
need to reconfigure.

Monitoring configuration based on  `discoveryRole` setting. By default, `endpoints` is used to get objects from kubernetes api. 
Its also possible to use `discoveryRole: service` or `discoveryRole: endpointslices`

 `Endpoints` objects are essentially lists of IP addresses. 
Typically, `Endpoints` objects are populated by `Service` object. `Service` object discovers `Pod`s by a label 
selector and adds those to the `Endpoints` object.

A `Service` may expose one or more service ports backed by a list of one or multiple endpoints pointing to 
specific `Pod`s. The same reflected in the respective `Endpoints` object as well.

The `VMServiceScrape` object discovers `Endpoints` objects and configures VMAgent to monitor `Pod`s.

The `Endpoints` section of the `VMServiceScrapeSpec` is used to configure which `Endpoints` ports should be scraped. 
For advanced use cases, one may want to monitor ports of backing `Pod`s, which are not a part of the service endpoints. 
Therefore, when specifying an endpoint in the `endpoints` section, they are strictly used.

> Note: `endpoints` (lowercase) is the field in the `VMServiceScrape` CRD, while `Endpoints` (capitalized) is the Kubernetes object kind.

Both `VMServiceScrape` and discovered targets may belong to any namespace. It is important for cross-namespace monitoring 
use cases, e.g. for meta-monitoring. Using the `serviceScrapeSelector` of the `VMAgentSpec` 
one can restrict the namespaces from which `VMServiceScrape`s are selected from by the respective VMAgent server. 
Using the `namespaceSelector` of the `VMServiceScrape` one can restrict the namespaces from which `Endpoints` can be 
discovered from. To discover targets in all namespaces the `namespaceSelector` has to be empty:

```yaml
spec:
  namespaceSelector: {}
```

## VMPodScrape

The `VMPodScrape` CRD allows to declaratively define how a dynamic set of pods should be monitored.
Use label selections to match pods for scraping. This allows an organization to introduce conventions 
for how metrics should be exposed. Following these conventions new services will be discovered automatically without 
need to reconfigure.

A `Pod` is a collection of one or more containers which can expose Prometheus metrics on a number of ports.

The `VMPodScrape` object discovers pods and generates the relevant scraping configuration. 

The `PodMetricsEndpoints` section of the `VMPodScrapeSpec` is used to configure which ports of a pod are going to be 
scraped for metrics and with which parameters.

Both `VMPodScrapes` and discovered targets may belong to any namespace. It is important for cross-namespace monitoring 
use cases, e.g. for meta-monitoring. Using the `namespaceSelector` of the `VMPodScrapeSpec` one can restrict the 
namespaces from which `Pods` are discovered from. To discover targets in all namespaces the `namespaceSelector` has to 
be empty:

```yaml
spec:
  namespaceSelector:
    any: true
```

## VMAlertmanager

The `VMAlertmanager` CRD declaratively defines a desired Alertmanager setup to run in a Kubernetes cluster. 
It provides options to configure replication and persistent storage.

For each `Alertmanager` resource, the Operator deploys a properly configured `StatefulSet` in the same namespace. 
The Alertmanager pods are configured to include a `Secret` called `<alertmanager-name>` which holds the used 
configuration file in the key `alertmanager.yaml`.

When there are two or more configured replicas the Operator runs the Alertmanager instances in high availability mode.

## VMAlertmanagerConfig

The `VMAlertmanagerConfig` provides way to configure `VMAlertmanager` configuration with CRD. It allows to define different configuration parts, 
which will be merged by operator into config. It behaves like other config parts - `VMServiceScrape` and etc.

## VMRule

The `VMRule` CRD declaratively defines a desired Prometheus rule to be consumed by one or more VMAlert instances. 

Alerts and recording rules can be saved and applied as YAML files, and dynamically loaded without requiring any restart.


## VMPrometheusConverter

By default, the Operator converts and updates existing prometheus-operator API objects:

`ServiceMonitor` into `VMServiceScrape`
`PodMonitor` into `VMPodScrape`
`PrometheusRule` into `VMRule`
`Probe` into `VMProbe`
Removing prometheus-operator API objects wouldn't delete any converted objects. So you can safely migrate or run 
two operators at the same time.
 
  
## VMProbe

 The `VMProbe` CRD provides probing target ability with a prober. The most common prober is [blackbox exporter](https://github.com/prometheus/blackbox_exporter). 
 By specifying configuration at CRD, operator generates config for `VMAgent` and syncs it. Its possible to use static targets 
 or use standard k8s discovery mechanism with `Ingress`. 
  You have to configure blackbox exporter before you can use this feature. The second requirement is `VMAgent` selectors, 
  it must match your `VMProbe` by label or namespace selector.

## VMNodeScrape

The `VMNodeScrape` CRD provides discovery mechanism for scraping metrics kubernetes nodes.
By specifying configuration at CRD, operator generates config for `VMAgent` and syncs it. Its useful for cadvisor scraping,
node-exporter or other node-based exporters. `VMAgent` nodeScrapeSelector must match `VMNodeScrape` labels.

## VMStaticScrape

The `VMStaticScrape` CRD provides mechanism for scraping metrics from static targets, configured by CRD targets.
By specifying configuration at CRD, operator generates config for `VMAgent` and syncs it. It's useful for external targets management,
when service-discovery is not available. `VMAgent` staticScrapeSelector must match `VMStaticScrape` labels.
 
## VMAuth

 The `VMAuth` CRD provides mechanism for exposing application with authorization to outside world or to other applications inside kubernetes cluster.
For first case, user can configure `ingress` setting at `VMAuth` CRD. For second one, operator will create secret with `username` and `password` at `VMUser` CRD name. 
So it will be possible to access this credentials from any application by targeting corresponding kubernetes secret.

## VMUser

 The `VMUser` CRD describes user configuration, its authentication methods `basic auth` or `Authorization` header. User access permissions, with possible routing information.
 User can define routing target with `static` config, by entering target `url`, or with `CRDRef`, in this case, operator queries kubernetes API, retrieves information about CRD and builds proper url.
