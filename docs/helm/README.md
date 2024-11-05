[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/victoriametrics)](https://artifacthub.io/packages/search?repo=victoriametrics&verified_publisher=true)
[![License](https://img.shields.io/github/license/VictoriaMetrics/VictoriaMetrics.svg)](https://github.com/VictoriaMetrics/helm-charts/blob/master/LICENSE)
![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)
[![Slack](https://img.shields.io/badge/join%20slack-%23victoriametrics-brightgreen.svg)](https://slack.victoriametrics.com/)

This repository contains helm charts for VictoriaMetrics and VictoriaLogs.

## Add a chart helm repository (can be skipped for OCI repository)

Access a Kubernetes cluster.

Add a chart helm repository with follow commands:

```console
helm repo add vm https://victoriametrics.github.io/helm-charts/

helm repo update
```

List [all charts](#list-of-charts) and versions of `vm` repository available to installation:
    
```console
helm search repo vm/
```

The command must display existing helm chart e.g.

```shell
NAME                            CHART VERSION   APP VERSION             DESCRIPTION
vm/victoria-logs-single         0.5.2           v0.15.0-victorialogs    Victoria Logs Single version - high-performance...
vm/victoria-metrics-agent       0.10.9          v1.101.0                Victoria Metrics Agent - collects metrics from ...
vm/victoria-metrics-alert       0.9.9           v1.101.0                Victoria Metrics Alert - executes a list of giv...
vm/victoria-metrics-anomaly     1.3.0           v1.13.0                 Victoria Metrics Anomaly Detection - a service ...
vm/victoria-metrics-auth        0.4.13          v1.101.0                Victoria Metrics Auth - is a simple auth proxy ...
vm/victoria-metrics-cluster     0.11.19         v1.101.0                Victoria Metrics Cluster version - high-perform...
vm/victoria-metrics-distributed 0.1.0           v1.101.0                A Helm chart for Running VMCluster on Multiple ...
vm/victoria-metrics-gateway     0.1.62          v1.101.0                Victoria Metrics Gateway - Auth & Rate-Limittin...
vm/victoria-metrics-k8s-stack   0.23.2          v1.101.0                Kubernetes monitoring on VictoriaMetrics stack....
vm/victoria-metrics-operator    0.32.2          v0.45.0                 Victoria Metrics Operator
vm/victoria-metrics-single      0.9.22          v1.101.0                Victoria Metrics Single version - high-performa...
```

## Installing the chart

Export default values of `victoria-metrics-cluster` chart to file `values.yaml`:

  - For HTTPS repository

    ```console
    helm show values vm/victoria-metrics-cluster > values.yaml
    ```
  - For OCI repository

    ```console
    helm show values oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-agent > values.yaml
    ```

Change the values according to the need of the environment in ``values.yaml`` file.

Test the installation with command:

  - For HTTPS repository

    ```console
    helm install victoria-metrics vm/victoria-metrics-cluster -f values.yaml -n NAMESPACE --debug --dry-run
    ```

  - For OCI repository

    ```console
    helm install victoria-metrics oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-cluster -f values.yaml -n NAMESPACE --debug --dry-run
    ```

Install chart with command:

  - For HTTPS repository
    
    ```console
    helm install victoria-metrics vm/victoria-metrics-cluster -f values.yaml -n NAMESPACE
    ```

  - For OCI repository

    ```console
    helm install victoria-metrics oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-cluster -f values.yaml -n NAMESPACE
    ```

## Validate installation

Get the pods lists by running these commands:

```console
kubectl get pods -A | grep 'victoria-metrics'

# or list all resorces of victoria-metrics

kubectl get all -n NAMESPACE | grep victoria
```

Get the application by running this commands:

```console
helm list -f victoria-metrics -n NAMESPACE
```

See the history of versions of ``victoria-metrics`` application with command.

```console
helm history victoria-metrics -n NAMESPACE
```

## How to uninstall VictoriaMetrics

Remove application with command.

```console
helm uninstall victoria-metrics -n NAMESPACE
```

## Kubernetes compatibility versions

helm charts tested at kubernetes versions from 1.28 to 1.30.

## List of Charts

- [VictoriaLogs Single](https://docs.victoriametrics.com/helm/victorialogs-single)
- [VictoriaMetrics Agent](https://docs.victoriametrics.com/helm/victoriametrics-agent)
- [VictoriaMetrics Alert](https://docs.victoriametrics.com/helm/victoriametrics-alert)
- [VictoriaMetrics Anomaly](https://docs.victoriametrics.com/helm/victoriametrics-anomaly)
- [VictoriaMetrics Auth](https://docs.victoriametrics.com/helm/victoriametrics-auth)
- [VictoriaMetrics Cluster](https://docs.victoriametrics.com/helm/victoriametrics-cluster)
- [VictoriaMetrics Gateway](https://docs.victoriametrics.com/helm/victoriametrics-gateway)
- [VictoriaMetrics Distributed](https://docs.victoriametrics.com/helm/victoriametrics-distributed)
- [VictoriaMetrics K8s Stack](https://docs.victoriametrics.com/helm/victoriametrics-k8s-stack)
- [VictoriaMetrics Operator](https://docs.victoriametrics.com/helm/victoriametrics-operator)
- [VictoriaMetrics Single](https://docs.victoriametrics.com/helm/victoriametrics-single)
