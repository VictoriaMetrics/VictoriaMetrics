---
sort: 1
---

# VictoriaMetrics operator

## Overview

Design and implementation inspired by [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator). It's great a tool for managing monitoring configuration of your applications. VictoriaMetrics operator has api capability with it.
So you can use familiar CRD objects: `ServiceMonitor`, `PodMonitor`, `PrometheusRule` and `Probe`. Or you can use VictoriaMetrics CRDs:
- `VMServiceScrape` - defines scraping metrics configuration from pods backed by services.
- `VMPodScrape` - defines scraping metrics configuration from pods.
- `VMRule` - defines alerting or recording rules.
- `VMProbe` - defines a probing configuration for targets with blackbox exporter.

Besides, operator allows you to manage VictoriaMetrics applications inside kubernetes cluster and simplifies this process [quick-start](https://docs.victoriametrics.com/operator/quick-start.html) 
With CRD (Custom Resource Definition) you can define application configuration and apply it to your cluster [crd-objects](https://docs.victoriametrics.com/operator/api.html). 

Operator simplifies VictoriaMetrics cluster installation, upgrading and managing.
 
It has integration with VictoriaMetrics `vmbackupmanager` - advanced tools for making backups. Check backup [docs](https://docs.victoriametrics.com/operator/backups.html)

## Use cases

For kubernetes-cluster administrators, it simplifies installation, configuration and management for `VictoriaMetrics` application. The main feature of operator is its ability to delegate the configuration of applications monitoring to the end-users.
 
For applications developers, its great possibility for managing observability of applications. You can define metrics scraping and alerting configuration for your application and manage it with an application deployment process. Just define app_deployment.yaml, app_vmpodscrape.yaml and app_vmrule.yaml. That's it, you can apply it to a kubernetes cluster. Check [quick-start](/Operator/quick-start.html) for an example.

## Operator vs helm-chart

VictoriaMetrics provides [helm charts](https://github.com/VictoriaMetrics/helm-charts). Operator makes the same, simplifies it and provides advanced features.


## Configuration

Operator configured by env variables, list of it can be found at [link](https://docs.victoriametrics.com/operator/vars.html)

It defines default configuration options, like images for components, timeouts, features.


## Kubernetes' compatibility versions

Operator tested at kubernetes versions from 1.16 to 1.22.

For clusters version below 1.16 you must use legacy CRDs from [path](https://github.com/VictoriaMetrics/operator/tree/master/config/crd/legacy) 
and disable CRD controller with flag: `--controller.disableCRDOwnership=true`

## Troubleshooting

- cannot apply crd at kubernetes 1.18 + version and kubectl reports error:
```console
Error from server (Invalid): error when creating "release/crds/crd.yaml": CustomResourceDefinition.apiextensions.k8s.io "vmalertmanagers.operator.victoriametrics.com" is invalid: [spec.validation.openAPIV3Schema.properties[spec].properties[initContainers].items.properties[ports].items.properties[protocol].default: Required value: this property is in x-kubernetes-list-map-keys, so it must have a default or be a required property, spec.validation.openAPIV3Schema.properties[spec].properties[containers].items.properties[ports].items.properties[protocol].default: Required value: this property is in x-kubernetes-list-map-keys, so it must have a default or be a required property]
Error from server (Invalid): error when creating "release/crds/crd.yaml": CustomResourceDefinition.apiextensions.k8s.io "vmalerts.operator.victoriametrics.com" is invalid: [
```
  upgrade to the latest release version. There is a bug with kubernetes objects at the early releases.


## Development

- [operator-sdk](https://github.com/operator-framework/operator-sdk) version v1.0.0+  
- golang 1.15 +
- minikube or kind

start:
```console
make run
```

for test execution run:
```console
#unit tests

make test 

# you need minikube or kind for e2e, do not run it on live cluster
#e2e tests with local binary
make e2e-local
```
