---
sort: 2
weight: 2
title: Setup
menu:
  docs:
    parent: "operator"
    weight: 2
aliases:
  - /operator/setup.html
---

# VictoriaMetrics Operator Setup

## Installing by helm-charts

You can use one of the following official helm-charts with `vmoperator`:

- [victoria-metrics-operator helm-chart](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-operator/README.md)
- [victoria-metrics-k8s-stack helm chart](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/README.md)
  (includes the `victoria-metrics-operator` helm-chart and other components for full-fledged k8s monitoring, is an alternative for [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)).

For installing VictoriaMetrics operator with helm-chart follow the instructions from README of the corresponding helm-chart
([this](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-operator/README.md)
or [this](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/README.md)).

in addition, you can use [quickstart guide](./quick-start.md) for 
installing VictoriaMetrics operator with helm-chart.

## Installing by Manifest

Obtain release from releases page:
[https://github.com/VictoriaMetrics/operator/releases](https://github.com/VictoriaMetrics/operator/releases)

We suggest use the latest release.

```sh
# Get latest release version from https://github.com/VictoriaMetrics/operator/releases/latest
export VM_VERSION=`basename $(curl -fs -o/dev/null -w %{redirect_url} https://github.com/VictoriaMetrics/operator/releases/latest)`
wget https://github.com/VictoriaMetrics/operator/releases/download/$VM_VERSION/bundle_crd.zip
unzip  bundle_crd.zip
```

Operator use `monitoring-system` namespace, but you can install it to specific namespace with command:

```sh
sed -i "s/namespace: monitoring-system/namespace: YOUR_NAMESPACE/g" release/operator/*
```

First of all, you  have to create [custom resource definitions](https://github.com/VictoriaMetrics/operator):

```sh
kubectl apply -f release/crds
```

Then you need RBAC for operator, relevant configuration for the release can be found at `release/operator/rbac.yaml`.

Change configuration for operator at `release/operator/manager.yaml`, possible settings: [operator-settings](/operator/vars.html)
and apply it:

```sh
kubectl apply -f release/operator/
```

Check the status of operator

```sh
kubectl get pods -n monitoring-system

#NAME                           READY   STATUS    RESTARTS   AGE
#vm-operator-667dfbff55-cbvkf   1/1     Running   0          101s
```

## Installing by Kustomize

You can install operator using [Kustomize](https://kustomize.io/) by pointing to the remote kustomization file.

```sh
# Get latest release version from https://github.com/VictoriaMetrics/operator/releases/latest
export VM_VERSION=`basename $(curl -fs -o/dev/null -w %{redirect_url} https://github.com/VictoriaMetrics/operator/releases/latest)`

cat << EOF > kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- github.com/VictoriaMetrics/operator/config/default?ref=${VM_VERSION}

images:
- name: victoriametrics/operator
  newTag: ${VM_VERSION}
EOF
```

You can change [operator configuration](#configuring), or use your custom namespace see [kustomize-example](https://github.com/YuriKravetc/yurikravetc.github.io/tree/main/Operator/kustomize-example).

Build template

```sh
kustomize build . -o monitoring.yaml
```

Apply manifests

```sh
kubectl apply -f monitoring.yaml
```

Check the status of operator

```sh
kubectl get pods -n monitoring-system

#NAME                           READY   STATUS    RESTARTS   AGE
#vm-operator-667dfbff55-cbvkf   1/1     Running   0          101s
```

## Installing to ARM

There is no need in an additional configuration for ARM. Operator and VictoriaMetrics have full support for it.

## Configuring

You can read detailed instructions about operator configuring in [this document](./configuration.md).
