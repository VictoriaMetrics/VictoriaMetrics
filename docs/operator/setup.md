---
weight: 2
title: Setup
menu:
  docs:
    parent: "operator"
    weight: 2
aliases:
  - /operator/setup/
  - /operator/setup/index.html
---
## Installing by helm-charts

You can use one of the following official helm-charts with `vmoperator`:

- [victoria-metrics-operator helm-chart](https://docs.victoriametrics.com/helm/victoriametrics-operator)
- [victoria-metrics-k8s-stack helm chart](https://docs.victoriametrics.com/helm/victoriametrics-k8s-stack)
  (includes the `victoria-metrics-operator` helm-chart and other components for full-fledged k8s monitoring, is an alternative for [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)).

For installing VictoriaMetrics operator with helm-chart follow the instructions from README of the corresponding helm-chart
([this](https://docs.victoriametrics.com/helm/victoriametrics-operator)
or [this](https://docs.victoriametrics.com/helm/victoriametrics-k8s-stack)).

in addition, you can use [quickstart guide](https://docs.victoriametrics.com/operator/quick-start) for
installing VictoriaMetrics operator with helm-chart.

## Installing by Manifest

Obtain release from releases page:
[https://github.com/VictoriaMetrics/operator/releases](https://github.com/VictoriaMetrics/operator/releases)

We suggest use the latest release.

```sh
# Get latest release version from https://github.com/VictoriaMetrics/operator/releases/latest
export VM_VERSION=`basename $(curl -fs -o/dev/null -w %{redirect_url} https://github.com/VictoriaMetrics/operator/releases/latest)`
wget https://github.com/VictoriaMetrics/operator/releases/download/$VM_VERSION/install.yaml
```

Operator use `vm` namespace, but you can install it to specific namespace with command:

```sh
sed -i "s/namespace: vm/namespace: YOUR_NAMESPACE/g" install.yaml
```

and apply it:

```sh
kubectl apply -f install.yaml
```

Check the status of operator

```sh
kubectl get pods -n YOUR_NAMESPACE

#NAME                           READY   STATUS    RESTARTS   AGE
#vm-operator-667dfbff55-cbvkf   1/1     Running   0          101s
```

## Installing by Kustomize

You can install operator using [Kustomize](https://kustomize.io/) by pointing to the remote kustomization file.

```sh
# Get latest release version from https://github.com/VictoriaMetrics/operator/releases/latest
export VM_VERSION=`basename $(curl -fs -o/dev/null -w %{redirect_url} https://github.com/VictoriaMetrics/operator/releases/latest)`
export NAMESPACE="whatever-namespace"

cat << EOF > kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- github.com/VictoriaMetrics/operator/config/base?ref=${VM_VERSION}

namespace: ${NAMESPACE}

images:
- name: manager
  newName: victoriametrics/operator
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
kubectl get pods -n whatever-namespace

#NAME                           READY   STATUS    RESTARTS   AGE
#vm-operator-667dfbff55-cbvkf   1/1     Running   0          101s
```

## Installing by OLM

### Installing to K8s

VictoriaMetrics operator OLM package is available at [OperatorHub](https://operatorhub.io/operator/victoriametrics-operator).
Installation instructions are available there.

### Installing to Openshift

Create `Subscription` manifest with `installPlanApproval` set to `Manual` to prevent unexpected upgrades.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: victoriametrics-operator
  namespace: vm
spec:
  channel: beta
  installPlanApproval: Manual
  name: victoriametrics-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
  startingCSV: victoriametrics-operator.v0.46.4
```

Apply manifest

```shell
oc apply -f manifest.yaml
```

After some time operator should be up and running in `vm` namespace

```shell
oc get pods -n vm
```

### Run locally

It's possible to build and run OLM package locally on Kind K8s cluster using `make deploy-kind-olm`.
Command builds operator image, bundle and index images, runs Kind with a local registry and deploys OLM package to Kind.

## Installing to ARM

There is no need in an additional configuration for ARM. Operator and VictoriaMetrics have full support for it.

## Configuring

You can read detailed instructions about operator configuring in [this document](https://docs.victoriametrics.com/operator/configuration).
