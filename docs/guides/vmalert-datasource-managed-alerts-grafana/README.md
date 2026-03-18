---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

Grafana offers a rich alerting UI and features like rule grouping, silences, and notification history. But while Grafana-native alerts are easy to use, they scale poorly since it depends on a relational database.

By moving rule evaluation to [vmalert], you can move past these limitations while retaining Grafana's unified alerting UI. This guide shows the ideal topology for scalable alerting using vmalert, Alertmanager, and Grafana datasource-managed alerts.

## Grafana Alert Modes

Grafana supports two alert modes, which can run side by side:

- Grafana-managed: alerts are created and evaluated entirely within Grafana itself. State is stored in a SQL database.
- Datasource-managed: alerts have their rules defined, stored, and evaluated in an external system like vmalert and Alertmanager, with Grafana just providing the UI. State is stored in VictoriaMetrics.

The following table compares the two modes:

| Aspect              | Grafana-Native              | Data Source-Managed           |
| ------------------- | --------------------------- | ----------------------------- |
| Where rules live    | Grafana database (SQL)      | vmalert's YAML config         |
| Evaluation          | Grafana's scheduler         | vmalert                       |
| Scaling             | Vertical (Grafana limits)   | Horizontal (vmalert shards)   |
| State storage       | SQL backend                 | VictoriaMetrics               |
| UI Management       | Full create/edit in Grafana | View-only                     |
| Dependencies        | SQL + Grafana               | Just VictoriaMetrics          |
| Rules can be <br/> version-controlled? | No             | Yes                           |

## Datasource-managed Alert Topology

The proposed alert setup relies on the following services:

- VictoriaMetrics: provides the time-series database and persists alert status.
- vmalert: evaluates alerting rules from its config file against VictoriaMetrics data and forwards firing alerts to Alertmanager.
- Alertmanager: groups and routes alerts to the configured recipients.
- Grafana: serves as the unified UI, connecting to VictoriaMetrics for rules and metrics, and to Alertmanager for notifications/silences.

TOPOLOGY DIAGRAM

## vmalert Demo with Docker

In this section, we'll describe how you can try datasource-managed alerts on Grafana with Docker Compose. Follow the steps in this section if you want to see how Grafana UI looks in datasource-managed alerts.

First, create `alerts.yml`. The following config creates an always-firing alert that works well in the Grafana UI demo:

```yaml
# alerts.yml
groups:
  - name: demo
    rules:
      # Always-firing demo alert so you see something immediately
      - alert: AlwaysFiring
        expr: vector(1)
        for: 10s
        labels:
          severity: warning
        annotations:
          summary: "Demo alert that always fires"
          description: "This is a demo alert from vmalert using vector(1)."

      # Simple recording rule you can graph in Grafana
      - record: demo:vector_one
        expr: vector(1)
```

Next, create a basic Alertmanager config called `alertmanager.yml`. This example does not forward alerts anywhere, but serves as a source for Grafana:

```yaml
# alertmanager.yml
global:
  resolve_timeout: 5m

route:
  receiver: "log"

receivers:
  - name: "log"
    webhook_configs:
      - url: "http://example.com"  # placeholder; replace with a real webhook later
```

Finally, create `grafana-datasources.yml` to configure Grafana to use VictoriaMetrics and Alertmanager as datasources for alerts and notifications:

```yaml
# grafana-datasources.yml
apiVersion: 1

datasources:
  - name: VictoriaMetrics
    type: prometheus
    access: proxy
    url: http://victoriametrics:8428
    isDefault: true

  - name: Alertmanager
    type: alertmanager
    access: proxy
    url: http://alertmanager:9093
    isDefault: false
    jsonData:
      implementation: prometheus
```

The final piece is the Docker Compose file. This ties all the services togheter, and set up the required command line arguments:

```yaml
services:
  victoriametrics:
    image: victoriametrics/victoria-metrics:latest
    container_name: victoriametrics
    command:
      - "--storageDataPath=/victoria-metrics-data"
      - "--retentionPeriod=30d"
      - "--selfScrapeInterval=10s"
      # Proxy vmalert APIs so Grafana can see rules via VictoriaMetrics
      - "--vmalert.proxyURL=http://vmalert:8880"
    ports:
      - "8428:8428"
    volumes:
      - vm-data:/victoria-metrics-data

  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    command:
      - "--config.file=/etc/alertmanager/alertmanager.yml"
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro

  vmalert:
    image: victoriametrics/vmalert:latest
    container_name: vmalert
    depends_on:
      - victoriametrics
      - alertmanager
    command:
      # read metrics from VictoriaMetrics
      - "--datasource.url=http://victoriametrics:8428"
      # store and retrieve alert state rom VictoriaMetrics
      - "--remoteRead.url=http://victoriametrics:8428"
      - "--remoteWrite.url=http://victoriametrics:8428"
      # configure Alertmanager as the notifier
      - "--notifier.url=http://alertmanager:9093"
      - "--rule=/etc/vmalert/alerts.yml"
      - "--evaluationInterval=15s"
      # external settings link vmalerts and Alertmanager to Grafana
      - "--external.url=http://localhost:3000"
      - "--external.alert.source=explore?orgId=1&left={\"datasource\":\"VictoriaMetrics\",\"queries\":[{\"refId\":\"A\",\"expr\":\"{{.Expr|queryEscape}}\"}]}"
    ports:
      - "8880:8880"
    volumes:
      - ./alerts.yml:/etc/vmalert/alerts.yml:ro

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    depends_on:
      - victoriametrics
      - alertmanager
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_PATHS_PROVISIONING: /etc/grafana/provisioning
    ports:
      - "3000:3000"
    volumes:
      - ./grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml:ro
      - grafana-data:/var/lib/grafana

volumes:
  vm-data:
  grafana-data:
```

Let's break down the main command line arguments that connect every component:

- VictoriaMetrics
  - `-vmalert.proxyURL`: forwards Grafana requests for `/api/v1/rules` and `/api/v1/alerts` to vmalert, enabling rule visibility in Grafana UI

- vmalert
  - `-datasource.url`: configures VictoriaMetrics as the query source for rule evaluation
  - `-remoteWrite.url`: defines VictoriaMetrics as the backend used to persist rule state across restarts
  - `-remoteRead.url`: defines VictoriaMetrics as the backend used to read historical state for pending alerts
  - `-notifier.url`: directs firing alerts to Alertmanager
  - `-external.url`: defines the base URL for alert links. Allows Alertmanager UI to link directly to Grafana
  - `-external.alert.source`: creates a template for clickable alert links for Grafana. Allows Alertmanager UI to link Directly to Grafana

Now, start the demo with:

```sh
docker compose up -d
```

Open your browser at `localhost:3000` and login to Grafana with username `admin` and password `admin`.

If you open the sidebar and select **Alerting** > **Alert rules**, you should be able to the see one alert pending or firing.

![Screenshot of Grafana alert pane](grafana-alert-firing.webp)
<figcaption style="text-align: center; font-style: italic;">Datasource-managed alert firing in Grafana</figcaption>

Open the sidebar again and go to **Alerting** > **Active notifications** to see the active alert reported by Alertmanager.

![Screenshot of Grafana Active notifications Page](grafana-active-notifications.webp)

You can also see the alerts in VMUI by opening the browser in `http://localhost:8428/vmui/?#/rules`. This is possible only when we have configured `-proxyURL` in VictoriaMetrics.

![Screenshot of VMUI](vmui-alerts.webp)
<figcaption style="text-align: center; font-style: italic;">Alerts can be visualized in VMUI directly</figcaption>

If you open the browser in `http://localhost:9093/#/alerts`, you will see the Alertmanager UI with the firing alert.

![Screenshot of Alertmanager](alertmanager-alerts.webp)
<figcaption style="text-align: center; font-style: italic;">Alertmanager UI showing the firing alert</figcaption>

Clicking on **Source** should take you back to Grafana and show you the query that originated the alert.

## vmalert and VictoriaMetrics Single on Kubernetes

This section explains how to configure datasource-managed alerts on VictoriaMetrics single-node version on Kubernetes.

### Prerequisites

- A Kubernetes cluster
- VictoriaMetrics single-node
- Grafana
- Helm values or config files used for installation
TODO: mention in the vmsingle and vmcluster that we should download and commit our config files

You can follow this guide to install all required components: [Kubernetes monitoring via VictoriaMetrics Single](https://docs.victoriametrics.com/guides/k8s-monitoring-via-vm-single/).

### 1. Ensure VictoriaMetrics and Grafana are installed

Ensure you have added and updated the VictoriaMetrics Helm repository:

```sh
helm repo add vm https://victoriametrics.github.io/helm-charts/
helm repo update

```

Confirm that VictoriaMetrics single-node version is installed with:

```sh
kubectl get pods -l app.kubernetes.io/instance=vmsingle
```

You should get a single running pod:

```sh
NAME                                        READY   STATUS    RESTARTS   AGE
vmsingle-victoria-metrics-single-server-0   1/1     Running   0          48m
```

Do the same for Grafana:

```sh
kubectl get pod -l app.kubernetes.io/name=grafana
```

You should get the name of the Grafana pod running in your cluster:

```sh
NAME                          READY   STATUS    RESTARTS   AGE
my-grafana-65d6d4ccbc-nxkxq   1/1     Running   0          58m
```

### 2. Install vmalert and Alertmanager

Create a Helm values files for vmalert and Alertmanager called `vm-alerting-values.yml`. 

The example below comes with two demo alerts. Add your own vmalert [alerting rules](https://docs.victoriametrics.com/victoriametrics/vmalert/#rules) in the `config: alerts:` section below.

```sh
cat <<EOF > vm-alerting-values.yml
# Enable and configure Alertmanager
alertmanager:
  enabled: true
  config:
    global:
      resolve_timeout: 5m
    route:
      group_by: ["alertname"]
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      receiver: "webhook"

    receivers:
      - name: "webhook"
        webhook_configs:
          - url: "http://example.com"  # placeholder; replace with a real webhook later
            send_resolved: true

# Configure vmalert ("server" section in this chart)
server:
  # vmalert evaluation datasource: point at vmsingle’s Prometheus-compatible API
  datasource:
    url: http://vmsingle-victoria-metrics-single-server.default.svc.cluster.local.:8428

  # Where vmalert stores and reads alert state (remote write/read)
  remote:
    write:
      url: http://vmsingle-victoria-metrics-single-server.default.svc.cluster.local.:8428
    read:
      url: http://vmsingle-victoria-metrics-single-server.default.svc.cluster.local.:8428

  # Configure Alertmanager as notifier
  notifier:
    alertmanager:
      # Adjust namespace/name if you install into a non-default namespace or change the release name
      url: http://vmalert-victoria-metrics-alert-alertmanager:9093

  # Inline demo rules. Add your alerting groups and rules here
  config:
    alerts:
      groups:
      - name: vm-health
        rules:
          # TODO: remove this and the recording rule below
          - alert: AlwaysFiring
            expr: vector(1)
            for: 10s
            labels:
              severity: warning
            annotations:
              summary: "Demo alert that always fires"
              description: "This is a demo alert from vmalert using vector(1)."

          # Simple recording rule you can graph in Grafana
          - record: demo:vector_one
            expr: vector(1)

          - alert: TooManyRestarts
            expr: changes(process_start_time_seconds{job=~"victoriametrics|vmagent|vmalert"}[15m]) > 2
            labels:
              severity: critical
            annotations:
              summary: "{{ $labels.job }} too many restarts (instance {{ $labels.instance }})"
              description: "Job {{ $labels.job }} has restarted more than twice in the last 15 minutes.
                It might be crashlooping."
          - alert: ServiceDown
            expr: up{job=~"victoriametrics|vmagent|vmalert"} == 0
            for: 2m
            labels:
              severity: critical
            annotations:
              summary: "Service {{ $labels.job }} is down on {{ $labels.instance }}"
              description: "{{ $labels.instance }} of job {{ $labels.job }} has been down for more than 2 minutes."
EOF
```

Install vmagent and Alertmanager with:

```sh
helm install vmalert vm/victoria-metrics-alert -f vm-alerting-values.yml
```

### 3. Configure VictoriaMetrics single

For this step, we must configure VictoriaMetrics to connect with vmalert by adding the `proxyURL` parameter.

First, check the service name for VictoriaMetrics single-node:

```sh
kubectl get svc -l app.kubernetes.io/instance=vmalert,app=server
```

Get the name of the vmalert service:

```sh
NAME                                    TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE
vmalert-victoria-metrics-alert-server   ClusterIP   None         <none>        8880/TCP   58m
```

Next, create a Helm values file with the internal Kubernetes URL for vmalert.

```sh
cat <<EOF > vm-vmalert-proxy-values.yml
# vm-vmalert-proxy-values.yaml
server:
  extraArgs:
    vmalert.proxyURL: http://vmalert-victoria-metrics-alert-server.default.svc.cluster.local:8880
EOF

```
The proxyURL follows the pattern: `http://<vmalert-service-name>.<namespace>.svc.cluster.local:<port>`

Update the configuration of VictoriaMetrics single-node by adding `vm-vmalert-proxy-values.yml` and *the original Helm values** used when VictoriaMetrics was installed in the first place. In the example below, this file is called `vmsingle-values-file.yml`:

```sh
helm upgrade vmsingle vm/victoria-metrics-single \
  -f vmsingle-values-file.yml \
  -f vm-vmalert-proxy-values.yml
```

### 3. Configure Grafana

The last step is to add Alertmanager to Grafana.

Get the service name for Alertmanager in your cluster:

```sh
kubectl get svc -l app.kubernetes.io/instance=vmalert,app=alertmanager
```

Take note of the service name:

```text
NAME                                          TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
vmalert-victoria-metrics-alert-alertmanager   ClusterIP   10.43.114.243   <none>        9093/TCP   68m
```

Log in into you Grafana dashboard and go to **Add Datasource**

SCREENSHOT

Select **Alertmanager** from the list.

SCREENSHOT

Fill in the following values:
  - Implementation: Prometheus
  - URL: `http://vmalert-victoria-metrics-alert-alertmanager.default.svc.cluster.local:9093`

Ensure the URL matches the Alertmanager service name you obtained earlier.

Press **Save & test**

Now Grafana should start showing alert rules and notifications in its UI.

## NOW DO THE CLUSTER

