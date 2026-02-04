---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

TODO: explain that cluster is not the recommended staring point. single is more performant and sufficient for most setups (look for blog posts and doc page)

**This guide covers:**

* The setup of a [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) in [Kubernetes](https://kubernetes.io/) via Helm charts
* How to scrape metrics from Kubernetes components using service discovery
* How to visualize stored data
* How to store metrics in [VictoriaMetrics](https://victoriametrics.com) tsdb

**Precondition** TODO: use newer GKE

We will use:

* [Kubernetes cluster 1.31.1-gke.1678000](https://cloud.google.com/kubernetes-engine)

> We use GKE cluster from [GCP](https://cloud.google.com/) but this guide is also applied on any Kubernetes cluster. For example [Amazon EKS](https://aws.amazon.com/ru/eks/).

* [Helm 4.1+](https://helm.sh/docs/intro/install)
* [kubectl 1.34+](https://kubernetes.io/docs/tasks/tools/install-kubectl)

![VMCluster on K8s](scheme.webp)

## 1. VictoriaMetrics Helm repository

You need to add the VictoriaMetrics Helm repository to install VictoriaMetrics components. We’re going to use [VictoriaMetrics Cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/). You can do this by running the following command:

```shell
helm repo add vm https://victoriametrics.github.io/helm-charts/
```

Update Helm repositories:

```shell
helm repo update
```

To verify that everything is set up correctly you may run this command:

```shell
helm search repo vm/
```

The expected output is:

```text
NAME                                    CHART VERSION   APP VERSION     DESCRIPTION
vm/victoria-logs-agent                  0.0.9           v1.44.0         VictoriaLogs Agent - accepts logs from various ...
vm/victoria-logs-cluster                0.0.26          v1.44.0         The VictoriaLogs cluster Helm chart deploys Vic...
vm/victoria-logs-collector              0.2.8           v1.44.0         VictoriaLogs Collector - collects logs from Kub...
vm/victoria-logs-multilevel             0.0.8           v1.44.0         The VictoriaLogs multilevel Helm chart deploys ...
vm/victoria-logs-single                 0.11.25         v1.44.0         The VictoriaLogs single Helm chart deploys Vict...
vm/victoria-metrics-agent               0.31.0          v1.135.0        VictoriaMetrics Agent - collects metrics from v...
vm/victoria-metrics-alert               0.31.0          v1.135.0        VictoriaMetrics Alert - executes a list of give...
vm/victoria-metrics-anomaly             1.12.9          v1.28.2         VictoriaMetrics Anomaly Detection - a service t...
vm/victoria-metrics-auth                0.24.0          v1.135.0        VictoriaMetrics Auth - is a simple auth proxy a...
vm/victoria-metrics-cluster             0.34.0          v1.135.0        VictoriaMetrics Cluster version - high-performa...
vm/victoria-metrics-common              0.0.46                          VictoriaMetrics Common - contains shared templa...
vm/victoria-metrics-distributed         0.29.0          v1.135.0        A Helm chart for Running VMCluster on Multiple ...
vm/victoria-metrics-gateway             0.22.0          v1.135.0        VictoriaMetrics Gateway - Auth & Rate-Limitting...
vm/victoria-metrics-k8s-stack           0.70.0          v1.135.0        Kubernetes monitoring on VictoriaMetrics stack....
vm/victoria-metrics-operator            0.58.1          v0.67.0         VictoriaMetrics Operator
vm/victoria-metrics-operator-crds       0.7.0           v0.67.0         VictoriaMetrics Operator CRDs
vm/victoria-metrics-single              0.30.0          v1.135.0        VictoriaMetrics Single version - high-performan...
vm/victoria-traces-cluster              0.0.6           v0.7.0          The VictoriaTraces cluster Helm chart deploys V...
vm/victoria-traces-single               0.0.6           v0.7.0          The VictoriaTraces single Helm chart deploys Vi...
```

## 2. Install VictoriaMetrics Cluster from the Helm chart

First, create a Helm values config file for VictoriaMetrics:

```sh
cat <<EOF >victoria-metrics-cluster-values.yml
vmselect:
  podAnnotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8481"

vminsert:
  podAnnotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8480"

vmstorage:
  podAnnotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8482"
EOF
```

The config file sets two settings for [vmselect], [vminsert], and [vmstorage]:

* `podAnnotations: prometheus.io/scrape: "true"` enables automatic service discovery and metric scraping from the VictoriaMetrics pods.
* `podAnnotations:prometheus.io/port: "some_port"` defines which port numbers to target for scraping metrics from the VictoriaMetrics pods.

Next, install VictoriaMetrics Cluster with the following command:

```sh
helm install vmcluster vm/victoria-metrics-cluster -f victoria-metrics-cluster-values.yml
```

As a result of this command you will see the following output:

```text
NAME: vmcluster
LAST DEPLOYED: Wed Feb  4 12:00:55 2026
NAMESPACE: default
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
NOTES:
Write API:

The Victoria Metrics write api can be accessed via port 8480 with the following DNS name from within your cluster:
vmcluster-victoria-metrics-cluster-vminsert.default.svc.cluster.local.

Get the Victoria Metrics insert service URL by running these commands in the same shell:
  export POD_NAME=$(kubectl get pods --namespace default -l "app=" -o jsonpath="{.items[0].metadata.name}")
  kubectl --namespace default port-forward $POD_NAME 8480

You need to update your Prometheus configuration file and add the following lines to it:

prometheus.yml

    remote_write:
      - url: "http://<insert-service>/insert/0/prometheus/"

for example -  inside the Kubernetes cluster:

    remote_write:
      - url: http://vmcluster-victoria-metrics-cluster-vminsert.default.svc.cluster.local.:8480/insert/0/prometheus/
Read API:

The VictoriaMetrics read api can be accessed via port 8481 with the following DNS name from within your cluster:
vmcluster-victoria-metrics-cluster-vmselect.default.svc.cluster.local.

Get the VictoriaMetrics select service URL by running these commands in the same shell:
  export POD_NAME=$(kubectl get pods --namespace default -l "app=" -o jsonpath="{.items[0].metadata.name}")
  kubectl --namespace default port-forward $POD_NAME 8481

You need to specify select service URL into your Grafana:
 NOTE: you need to use the Prometheus Data Source

Input this URL field into Grafana

    http://<select-service>/select/0/prometheus/


for example - inside the Kubernetes cluster:

    http://vmcluster-victoria-metrics-cluster-vmselect.default.svc.cluster.local.:8481/select/0/prometheus/
```

Note the URL for the `remote_write`. We are going to use this value for [Step 3](#id-3-install-vmagent-from-the-helm-chart) and [Step 4](#id-4-install-and-connect-grafana-to-victoriametrics-with-helm).

Verify that [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) pods are up and running by executing the following command:

```sh
kubectl get pods
```

The expected output is:

```text
NAME                                                           READY   STATUS    RESTARTS   AGE
vmcluster-victoria-metrics-cluster-vminsert-689cbc8f55-95szg   1/1     Running   0          16m
vmcluster-victoria-metrics-cluster-vminsert-689cbc8f55-f852l   1/1     Running   0          16m
vmcluster-victoria-metrics-cluster-vmselect-977d74cdf-bbgp5    1/1     Running   0          16m
vmcluster-victoria-metrics-cluster-vmselect-977d74cdf-vzp6z    1/1     Running   0          16m
vmcluster-victoria-metrics-cluster-vmstorage-0                 1/1     Running   0          16m
vmcluster-victoria-metrics-cluster-vmstorage-1                 1/1     Running   0          16m
```

## 3. Install vmagent from the Helm chart

To scrape metrics from Kubernetes with a [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) we need to install [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) with additional configuration. To do so, please run these commands in your terminal:

```shell
helm install vmagent vm/victoria-metrics-agent -f https://docs.victoriametrics.com/guides/examples/guide-vmcluster-vmagent-values.yaml
```

Here is full file content `guide-vmcluster-vmagent-values.yaml`

```yaml
remoteWrite:
  - url: http://vmcluster-victoria-metrics-cluster-vminsert.default.svc.cluster.local:8480/insert/0/prometheus/

config:
  global:
    scrape_interval: 10s

  scrape_configs:
    - job_name: vmagent
      static_configs:
        - targets: ["localhost:8429"]
    - job_name: "kubernetes-apiservers"
      kubernetes_sd_configs:
        - role: endpoints
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        insecure_skip_verify: true
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      relabel_configs:
        - source_labels:
            [
              __meta_kubernetes_namespace,
              __meta_kubernetes_service_name,
              __meta_kubernetes_endpoint_port_name,
            ]
          action: keep
          regex: default;kubernetes;https
    - job_name: "kubernetes-nodes"
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        insecure_skip_verify: true
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      kubernetes_sd_configs:
        - role: node
      relabel_configs:
        - action: labelmap
          regex: __meta_kubernetes_node_label_(.+)
    - job_name: "kubernetes-nodes-cadvisor"
      scheme: https
      tls_config:
        ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        insecure_skip_verify: true
      bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
      kubernetes_sd_configs:
        - role: node
      metrics_path: /metrics/cadvisor
      relabel_configs:
        - action: labelmap
          regex: __meta_kubernetes_node_label_(.+)
        - source_labels: [__metrics_path__]
          target_label: metrics_path
      metric_relabel_configs:
        - action: replace
          source_labels: [pod]
          regex: '(.+)'
          target_label: pod_name
          replacement: '${1}'
        - action: replace
          source_labels: [container]
          regex: '(.+)'
          target_label: container_name
          replacement: '${1}'
        - action: replace
          target_label: name
          replacement: k8s_stub
        - action: replace
          source_labels: [id]
          regex: '^/system\.slice/(.+)\.service$'
          target_label: systemd_service_name
          replacement: '${1}'
    - job_name: "kubernetes-service-endpoints"
      kubernetes_sd_configs:
        - role: endpoints
      relabel_configs:
        - action: drop
          source_labels: [__meta_kubernetes_pod_container_init]
          regex: true
        - action: keep_if_equal
          source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_container_port_number]
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_scrape]
          action: keep
          regex: true
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_scheme]
          action: replace
          target_label: __scheme__
          regex: (https?)
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)
        - source_labels:
            [
              __address__,
              __meta_kubernetes_service_annotation_prometheus_io_port,
            ]
          action: replace
          target_label: __address__
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
        - action: labelmap
          regex: __meta_kubernetes_service_label_(.+)
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: kubernetes_namespace
        - source_labels: [__meta_kubernetes_service_name]
          action: replace
          target_label: kubernetes_name
        - source_labels: [__meta_kubernetes_pod_node_name]
          action: replace
          target_label: kubernetes_node
    - job_name: "kubernetes-service-endpoints-slow"
      scrape_interval: 5m
      scrape_timeout: 30s
      kubernetes_sd_configs:
        - role: endpoints
      relabel_configs:
        - action: drop
          source_labels: [__meta_kubernetes_pod_container_init]
          regex: true
        - action: keep_if_equal
          source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_container_port_number]
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_scrape_slow]
          action: keep
          regex: true
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_scheme]
          action: replace
          target_label: __scheme__
          regex: (https?)
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)
        - source_labels:
            [
              __address__,
              __meta_kubernetes_service_annotation_prometheus_io_port,
            ]
          action: replace
          target_label: __address__
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
        - action: labelmap
          regex: __meta_kubernetes_service_label_(.+)
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: kubernetes_namespace
        - source_labels: [__meta_kubernetes_service_name]
          action: replace
          target_label: kubernetes_name
        - source_labels: [__meta_kubernetes_pod_node_name]
          action: replace
          target_label: kubernetes_node
    - job_name: "kubernetes-services"
      metrics_path: /probe
      params:
        module: [http_2xx]
      kubernetes_sd_configs:
        - role: service
      relabel_configs:
        - source_labels:
            [__meta_kubernetes_service_annotation_prometheus_io_probe]
          action: keep
          regex: true
        - source_labels: [__address__]
          target_label: __param_target
        - target_label: __address__
          replacement: blackbox
        - source_labels: [__param_target]
          target_label: instance
        - action: labelmap
          regex: __meta_kubernetes_service_label_(.+)
        - source_labels: [__meta_kubernetes_namespace]
          target_label: kubernetes_namespace
        - source_labels: [__meta_kubernetes_service_name]
          target_label: kubernetes_name
    - job_name: "kubernetes-pods"
      kubernetes_sd_configs:
        - role: pod
      relabel_configs:
        - action: drop
          source_labels: [__meta_kubernetes_pod_container_init]
          regex: true
        - action: keep_if_equal
          source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_container_port_number]
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: true
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)
        - source_labels:
            [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
          action: replace
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
          target_label: __address__
        - action: labelmap
          regex: __meta_kubernetes_pod_label_(.+)
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: kubernetes_namespace
        - source_labels: [__meta_kubernetes_pod_name]
          action: replace
          target_label: kubernetes_pod_name
```

The key settings in configuration file above are:

* `remoteWrite` defines the `vminsert` endpoint that receives telemetry from [vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/). This is the same URL for the `remote_write` in the output of [Step 2](#id-2-install-victoriametrics-cluster-from-the-helm-chart).
* `metric_relabel_configs` are the label rewriting rules that helps us show Kubernetes metrics in the Grafana dashboard.

Verify that `vmagent`'s pod is up and running by executing the following command:

```shell
kubectl get pods | grep vmagent
```

The expected output is:

```text
vmagent-victoria-metrics-agent-69974b95b4-mhjph                1/1     Running   0          11m
```

## 4. Install and connect Grafana to VictoriaMetrics with Helm

Add the Grafana Helm repository:

```shell
helm repo add grafana-community https://grafana-community.github.io/helm-charts
helm repo update
```

> [!NOTE] Tip
> See more information on Grafana in [ArtifactHUB](https://artifacthub.io/packages/helm/grafana-community/grafana)

Create a values config file to define the data sources and dashboards for VictoriaMetrics in the Grafana service:

```sh
cat <<EOF > grafana-cluster-values.yml
  datasources:
    datasources.yaml:
      apiVersion: 1
      datasources:
        - name: victoriametrics
          type: prometheus
          orgId: 1
          url: http://vmcluster-victoria-metrics-cluster-vmselect.default.svc.cluster.local:8481/select/0/prometheus/
          access: proxy
          isDefault: true
          updateIntervalSeconds: 10
          editable: true

  dashboardProviders:
   dashboardproviders.yaml:
     apiVersion: 1
     providers:
     - name: 'default'
       orgId: 1
       folder: ''
       type: file
       disableDeletion: true
       editable: true
       options:
         path: /var/lib/grafana/dashboards/default

  dashboards:
    default:
      victoriametrics:
        gnetId: 11176
        revision: 18
        datasource: victoriametrics
      vmagent:
        gnetId: 12683
        revision: 7
        datasource: victoriametrics
      kubernetes:
        gnetId: 14205
        revision: 1
        datasource: victoriametrics
EOF
```

The config file defines the following settings for Grafana:

* Provisions a VictoriaMetrics data source. This is the `remote_write` URL we obtained in [Step 2](#id-2-install-victoriametrics-cluster-from-the-helm-chart) when we installed the VictoriaMetrics Cluster.
* Add three starter dashboards:
  * [VictoriaMetrics - cluster](https://grafana.com/grafana/dashboards/11176-victoriametrics-cluster/) for the [VictoriaMetrics Cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/).
  * [VictoriaMetrics - vmagent](https://grafana.com/grafana/dashboards/12683-victoriametrics-vmagent/) for the [VictoriaMetrics Agent](https://docs.victoriametrics.com/victoriametrics/vmagent/).
  * [Kubernetes Cluster Monitoring (via Prometheus)](https://grafana.com/grafana/dashboards/14205-kubernetes-cluster-monitoring-via-prometheus/) to show Kubernetes cluster metrics.

Run the following command to install the Grafana chart with the name `my-grafana`:

```sh
helm install my-grafana grafana-community/grafana -f grafana-cluster-values.yml
```

You should get the following output:

```text
NAME: my-grafana
LAST DEPLOYED: Wed Feb  4 15:00:28 2026
NAMESPACE: default
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
NOTES:
1. Get your 'admin' user password by running:

   kubectl get secret --namespace default my-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo


2. The Grafana server can be accessed via port 80 on the following DNS name from within your cluster:

   my-grafana.default.svc.cluster.local

   Get the Grafana URL to visit by running these commands in the same shell:
     export POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=my-grafana" -o jsonpath="{.items[0].metadata.name}")
     kubectl --namespace default port-forward $POD_NAME 3000

3. Login with the password from step 1 and the username: admin
#################################################################################
######   WARNING: Persistence is disabled!!! You will lose your data when   #####
######            the Grafana pod is terminated.                            #####
#################################################################################
```

Use the first command in the output to obtain the password for the `admin` user:

```shell
kubectl get secret --namespace default my-grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo

```

The second part of the output shows how to port-forward the Grafana service in order to access it locally on `127.0.0.1:3000`:

```shell
export pod_name=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=my-grafana" -o jsonpath="{.items[0].metadata.name}")

kubectl --namespace default port-forward $pod_name 3000
```

## 5. Check the result you obtained in your browser

To check that [VictoriaMetrics](https://victoriametrics.com) collects metrics from the Kubernetes cluster open in browser [http://127.0.0.1:3000/dashboards](http://127.0.0.1:3000/dashboards) and choose the `Kubernetes Cluster Monitoring (via Prometheus)` dashboard.

Use `admin` for login and `password` obtained in the previous step.

You should see three dashboards installed. Select "Kubernetes Cluster Monitoring".

![Dashboards](dashes-agent.webp)

This is the main dashboard, which shows activity across your Kubernetes cluster:

![Kubernetes Cluster Dashboard](dashboard.webp)

The VictoriaMetrics Cluster dashboard is also available to monitor telemetry ingestion and resource utilization:

![VMCluster dashboard](grafana-dash-vmcluster.webp)

And vmagent has a separate dashboard to monitor scraping and queue activity:

![VMAgent dashboard](grafana-dash-vmagent.webp)

## 6. Final thoughts

* We set up TimeSeries Database for your Kubernetes cluster.
* We collected metrics from all running pods,nodes, … and stored them in a VictoriaMetrics database.
* We visualized resources used in the Kubernetes cluster by using Grafana dashboards.
