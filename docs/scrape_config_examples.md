---
sort: 200
weight: 200
title: Scrape config examples
menu:
  docs:
    parent: 'victoriametrics'
    weight: 200
aliases:
- /scrape_config_examples.html
---

# Scrape config examples

- [Static configs](#static-configs)
- [File-based target discovery](#file-based-target-discovery)
- [HTTP-based target discovery](#http-based-target-discovery)
- [Kubernetes target discovery](#kubernetes-target-discovery)


## Static configs

Let's start from a simple case with scraping targets at pre-defined addresses.
Create a `scrape.yaml` file with the following contents:

```yaml
scrape_configs:
- job_name: node-exporter
  static_configs:
  - targets:
    - localhost:9100
```

After you created the `scrape.yaml` file, download and unpack [single-node VictoriaMetrics](https://docs.victoriametrics.com/) to the same directory:

```
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.101.0/victoria-metrics-linux-amd64-v1.101.0.tar.gz
tar xzf victoria-metrics-linux-amd64-v1.101.0.tar.gz
```

Then start VictoriaMetrics and instruct it to scrape targets defined in `scrape.yaml` and save scraped metrics
to local storage according to [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter):

```
./victoria-metrics-prod -promscrape.config=scrape.yaml
```

Now open the `http://localhost:8428/targets` page in web browser in order to see the current status for scrape targets.
The page must contain the information about the target at `http://localhost:9100/metrics` url.
It is likely the target has `state: down` if you didn't start [`node-exporter`](https://github.com/prometheus/node_exporter) on `localhost`.

Let's add a new scrape config to `scrape.yaml` for scraping [VictoriaMetrics metrics](https://docs.victoriametrics.com/#monitoring):

```yaml
scrape_configs:
- job_name: node-exporter
  static_configs:
  - targets:
    - localhost:9100
- job_name: victoriametrics
  static_configs:
  - targets:
    - http://localhost:8428/metrics
```

Note that the last specified target contains the full url instead of host and port.
This is an extension supported by VictoriaMetrics and [vmagent](https://docs.victoriametrics.com/vmagent/) - you can use both `host:port`
and full urls in scrape target lists.

Send `SIGHUP` signal `victoria-metrics-prod` process, so it [reloads the updated `scrape.yaml`](https://docs.victoriametrics.com/vmagent/#configuration-update):

```
kill -HUP `pidof victoria-metrics-prod`
```

Now the `http://localhost:8428/targets` page must contain two targets - `http://localhost:9100/metrics` and `http://localhost:8428/metrics`.
The last one should have `state: up`, since this is VictoriaMetrics itself.

Let's query the scraped metrics. Open `http://localhost:8428/vmui/` aka [vmui](https://docs.victoriametrics.com/#vmui), enter `up` in the query input field
and press `enter`. You'll see a graph for `up` metrics. It must contain two lines for the targets defined in `scrape.yaml` file above.
See [these docs](https://docs.victoriametrics.com/vmagent/#automatically-generated-metrics) about `up` metric. You can explore other scraped metrics
in `vmui` via [Prometheus metrics explorer](https://docs.victoriametrics.com/#metrics-explorer).

Let's look closely to the contents of the `scrape.yaml` file created above:

```yaml
scrape_configs:
- job_name: node-exporter
  static_configs:
  - targets:
    - localhost:9100
- job_name: victoriametrics
  static_configs:
  - targets:
    - http://localhost:8428/metrics
```

The [`scrape_configs`](https://docs.victoriametrics.com/sd_configs/#scrape_configs) section contains a list of scrape configs.
Our `scrape.yaml` file contains two scrape configs - for `job_name: node-exporter` and for `job_name: victoriametrics`.
[vmagent](https://docs.victoriametrics.com/vmagent/) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
can efficiently process thousands of scrape configs in production.

Every scrape config in the list **must** contain `job_name` field - its' value is used as [`job`](https://prometheus.io/docs/concepts/jobs_instances/) label
in all the metrics scraped from targets defined in this scrape config.
Every scrape config must contain at least a single section from [this list](https://docs.victoriametrics.com/sd_configs/#supported-service-discovery-configs).
Every scrape config may contain other options described [here](https://docs.victoriametrics.com/sd_configs/#scrape_configs).

In our case only [`static_configs`](https://docs.victoriametrics.com/sd_configs/#static_configs) sections are used.
These sections consist of a list of static configs according to [these docs](https://docs.victoriametrics.com/sd_configs/#static_configs).
Every static config contains a list of `targets`, which need to be scraped. The target address is used as [`instance`](https://prometheus.io/docs/concepts/jobs_instances/)
label in all the metrics scraped from the target.

[vmagent](https://docs.victoriametrics.com/vmagent/) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
can efficiently process tens of thousands of targets in production. If you need scraping more targets,
then see [these docs](https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets).

Targets are scraped at `http` or `https` urls, which are formed according to [these rules](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets).
It is possible to modify scrape urls via [relabeling](https://docs.victoriametrics.com/relabeling/) if needed.


## File-based target discovery

It may be not so convenient updating `scrape.yaml` file with [`static_configs`](https://docs.victoriametrics.com/sd_configs/#static_configs)
every time new scrape target is added, changed or removed. In this case [`file_sd_configs`](https://docs.victoriametrics.com/sd_configs/#file_sd_configs)
can come to rescue. It allows defining a list of scrape targets in `JSON` files, and automatically updating the list of scrape targets
at [vmagent](https://docs.victoriametrics.com/vmagent/) or [single-node VictoriaMetrics](https://docs.victoriametrics.com/) side
when the corresponding `JSON` files are updated.

Let's create `node_exporter_targets.json` file with the following contents:

```json
[
  {
    "targets": ["host1:9100", "host2:9100"]
  }
]
```

Then create `scrape.yaml` file with the following contents:

```yaml
scrape_configs:
- job_name: node-exporter
  file_sd_configs:
  - files:
    - node_exporter_targets.json
```

Then start [single-node VictoriaMetrics](https://docs.victoriametrics.com/) according to [these docs](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter):

```yaml
# Download and unpack single-node VictoriaMetrics
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.101.0/victoria-metrics-linux-amd64-v1.101.0.tar.gz
tar xzf victoria-metrics-linux-amd64-v1.101.0.tar.gz

# Run single-node VictoriaMetrics with the given scrape.yaml
./victoria-metrics-prod -promscrape.config=scrape.yaml
```

Then open `http://localhost:8428/targets` page in web browser and see that it contains the two targets defined in `node_exporter_targets.json` above.

Now let's add more targets to `node_exporter_targets.json`:

```json
[
  {
    "targets": ["host1:9100", "host2:9100", "http://host3:9100/metrics", "http://host4:9100/metrics"]
  }
]
```

Note that the added targets contains full urls instead of host and port.
This is an extension supported by VictoriaMetrics and [vmagent](https://docs.victoriametrics.com/vmagent/) - you can use both `host:port`
and full urls in scrape target lists.

Save the updated `node_exporter_targets.json`, wait for 30 seconds and then refresh the `http://localhost:8428/targets` page.
Now this page must contain all the targets defined in the updated `node_exporter_targets.json`.
By default [vmagent](https://docs.victoriametrics.com/vmagent/) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
check for updates in `files` specified at [`file_sd_configs`](https://docs.victoriametrics.com/sd_configs/#file_sd_configs)
every 30 seconds. This interval can be changed via `-promscrape.fileSDCheckInterval` command-line flag.
For example, the following command starts VictoriaMetrics, which checks for updates in `file_sd_configs` every 5 seconds:

```
./victoria-metrics-prod -promscrape.config=scrape.yaml -promscrape.fileSDCheckInterval=5s
```

If the `files` contents is broken during the check, then the previous list of scrape targets is kept.

It is possible specifying `http` and/or `https` urls in `files` list. For example, the following config instructs
obtaining fresh list of targets at `http://central-config-server/targets?type=node-exporter` url
additionally to `node_exporter_targets.json` local file:

```yaml
scrape_configs:
- job_name: node-exporter
  file_sd_configs:
  - files:
    - node_exporter_targets.json
    - 'http://central-config-server/targets?type=node-exporter'
```

It is possible to specify directories with `*` wildcards for distinct sets of targets at `file_sd_configs`.
See [these docs](https://docs.victoriametrics.com/sd_configs/#file_sd_configs) for details.

[vmagent](https://docs.victoriametrics.com/vmagent/) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/)
can efficiently scrape tens of thousands of scrape targets. If you need scraping more targets,
then see [these docs](https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets).

Targets are scraped at `http` or `https` urls, which are formed according to [these rules](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets).
It is possible to modify scrape urls via [relabeling](https://docs.victoriametrics.com/relabeling/) if needed.


## HTTP-based target discovery

It may not so convenient maintaining a list of local files for [`file_sd_configs`](https://docs.victoriametrics.com/sd_configs/#file_sd_configs).
In this case [`http_sd_configs`](https://docs.victoriametrics.com/sd_configs/#http_sd_configs) can help.
They allow specifying a list of `http` or `https` urls, which return targets, which need to be scraped.
For example, the following [`-promscrape.config`](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
periodically fetches the list of targets from the specified url:

```yaml
scrape_configs:
- job_name: node-exporter
  http_sd_configs:
  - url: "http://central-config-server/targets?type=node-exporter"
```

## Kubernetes target discovery

Kubernetes target discovery is non-trivial task in general. That's why it is recommended using
either [victoria-metrics-k8s-stack Helm chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-k8s-stack)
or [VictoriaMetrics operator for Kubernetes](https://github.com/VictoriaMetrics/operator)
for Kubernetes monitoring.

If you feel brave, let's look at a few typical cases for Kubernetes monitoring.

### Discovering and scraping `node-exporter` targets in Kubernetes

The following [`-promscrape.config`](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
instructs discovering and scraping all the [`node-exporter`](https://github.com/prometheus/node_exporter) targets inside Kubernetes cluster:

```yaml
scrape_configs:
- job_name: node-exporter
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:

    # Leave only targets with `node-exporter` container name.
    # If node-exporter containers have another name in your Kubernetes cluster,
    # then adjust the regex value accordingly.
    #
  - source_labels: [__meta_kubernetes_pod_container_name]
    regex: node-exporter
    action: keep

    # Copy node name into `node` label, so node-exporter targets
    # can be attributed to a particular node.
    #
  - source_labels: [__meta_kubernetes_pod_node_name]
    target_label: node
```

See [`kubernetes_sd_configs` docs](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) for more details.

See [relabeling docs](https://docs.victoriametrics.com/vmagent/#relabeling) for details on `relabel_configs`.

### Discovering and scraping `kube-state-metrics` in Kubernetes

[kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) is a special metrics exporter,
which exposes `state` metrics for all the Kubernetes objects such as `container`, `pod`, `node`, etc.
It already sets `namespace`, `container`, `pod` and `node` labels for every exposed metric,
so these metrics shouldn't be set in [target relabeling](https://docs.victoriametrics.com/vmagent/#relabeling).

The following [`-promscrape.config`](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
instructs discovering and scraping [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) target inside Kubernetes cluster:

```yaml
scrape_configs:
- job_name: kube-state-metrics
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:

    # Leave only targets with `kube-state-metrics` container name.
    # If kube-state-metrics container has another name in your Kubernetes cluster,
    # then adjust the regex value accordingly.
  - source_labels: [__meta_kubernetes_pod_container_name]
    regex: kube-state-metrics
    action: keep

    # kube-state-metrics container may expose multiple ports.
    # We need scraping only the e.g. service port, and do not need scraping e.g. telemetry port.
    # The kube-state-metrics service port usually equals to 8080.
    # Modify the regex accordingly if you use other port for kube-state-metrics.
    #
  - source_labels: [__meta_kubernetes_pod_container_port_number]
    regex: "8080"
    action: keep
```

See [`kubernetes_sd_configs` docs](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) for more details.

See [relabeling docs](https://docs.victoriametrics.com/vmagent/#relabeling) for details on `relabel_configs`.

### Discovering and scraping metrics from `cadvisor`

[cadvisor](https://github.com/google/cadvisor) exposes resource usage metrics for every container in Kubernetes.
The following [`-promscrape.config`](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
can be used for collecting `cadvisor` metrics in Kubernetes:

```yaml
scrape_configs:
- job_name: cadvisor
  kubernetes_sd_configs:
    # Cadvisor is installed on every Kubernetes node, so use `role: node` service discovery
    #
  - role: node

  # This is needed for scraping cadvisor metrics from Kubernetes API server proxy.
  # See relabel_configs below.
  #
  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt

  relabel_configs:
    # Cadvisor metrics are better to scrape from Kubernetes API server proxy.
    # There is no need to add container, pod and node labels to the scraped metrics,
    # since cadvisor adds these labels on itself.
    #
  - source_labels: [__meta_kubernetes_node_name]
    target_label: __address__
    regex: '(.+)'
    replacement: https://kubernetes.default.svc/api/v1/nodes/$1/proxy/metrics/cadvisor
  - source_labels: [__meta_kubernetes_node_name]
    target_label: instance
```

See [`kubernetes_sd_configs` docs](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) for more details.

See [relabeling docs](https://docs.victoriametrics.com/vmagent/#relabeling) for details on `relabel_configs`.

See [these docs](https://docs.victoriametrics.com/sd_configs/#http-api-client-options) for details on `bearer_token_file` and `tls_config` options.

### Discovering and scraping metrics for a particular container in Kubernetes

The following [`-promscrape.config`](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
instructs discovering and scraping metrics for all the containers with the name `my-super-app`.
It is expected that these containers expose only a single TCP port, which serves its metrics at `/metrics` page
according to [Prometheus text exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format):

```yaml
scrape_configs:
- job_name: my-super-app
  kubernetes_sd_configs:
  - role: pod
  relabel_configs:

    # Leave only targets with the container name, which matches the `job_name` specified above
    # See https://docs.victoriametrics.com/relabeling/#how-to-modify-instance-and-job for details on `job` label.
    #
  - source_labels: [__meta_kubernetes_pod_container_name]
    target_label: job
    action: keepequal

    # Keep namespace, node, pod and container labels, so they can be used
    # for joining additional `state` labels exposed by kube-state-metrics
    # for the particular target.
    #
  - source_labels: [__meta_kubernetes_namespace]
    target_label: namespace
  - source_labels: [__meta_kubernetes_pod_node_name]
    target_label: node
  - source_labels: [__meta_kubernetes_pod_name]
    target_label: pod
  - source_labels: [__meta_kubernetes_pod_container_name]
    target_label: container
```

See [`kubernetes_sd_configs` docs](https://docs.victoriametrics.com/sd_configs/#kubernetes_sd_configs) for more details.

See [relabeling docs](https://docs.victoriametrics.com/vmagent/#relabeling) for details on `relabel_configs`.
