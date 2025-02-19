VictoriaMetrics Cloud is a managed, easy to use monitoring solution that integrates seamlessly with
other tools and frameworks in the Observability ecosystem such as OpenTelemetry, Grafana, Prometheus, Graphite,
InfluxDB, OpenTSDB and DataDog - see [these docs](https://docs.victoriametrics.com/#how-to-import-time-series-data)
for further details.

<br>
<!--TODO: Just a test: Needs to be changed by something better!-->

![](/victoriametrics-cloud/get-started/get_started_preview.webp)
<br>

## Get Started
* [Quick Start](/victoriametrics-cloud/quickstart/) documentation.
* [Try it now](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_overview) with a free trial.

## Use cases
The most common use cases for VictoriaMetrics Cloud are:
* Long-term remote storage for Prometheus metrics.
* Reliable and efficient drop-in replacement for Prometheus and Graphite.
* Efficient replacement for InfluxDB and OpenTSDB by consuming lower amounts of RAM, CPU and disk.
* Cost-efficient alternative for Observability services like DataDog.

## Benefits
We run VictoriaMetrics Cloud deployments in our environment on AWS and provide easy-to-use endpoints
for data ingestion and querying. The VictoriaMetrics team takes care of optimal configuration and software
maintenance. This means that VictoriaMetrics Cloud allows users to run the Enterprise version of VictoriaMetrics, hosted on AWS,
without the hustle to perform typical DevOps tasks such as:
* Managing configuration.
* Monitoring.
* Logs collection.
* Access protection.
* Software updates.
* Regular backups.
* Control costs.

## Features
VictoriaMetrics Cloud comes with the following features:
* It can be used as a Managed Prometheus - just configure Prometheus or vmagent to write data to VictoriaMetrics Cloud and then use the provided endpoint as a Prometheus datasource in Grafana.
* Built-in [Alerting & Recording](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/#configure-alerting-rules) rules execution.
* Hosted [Alertmanager](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/) for sending notifications.
* Every VictoriaMetrics Cloud deployment runs in an isolated environment, so deployments cannot interfere with each other.
* VictoriaMetrics Cloud deployment can be scaled up or scaled down in a few clicks.
* Automated backups.
* No surprises. Select a tier and pay only for the actual used resources - compute, storage, traffic.

## Learn more
* [VictoriaMetrics Cloud announcement](https://victoriametrics.com/blog/introduction-to-managed-monitoring/).
* [Pricing comparison for Managed Prometheus](https://victoriametrics.com/blog/managed-prometheus-pricing/).
* [Monitoring Proxmox VE via VictoriaMetrics Cloud and vmagent](https://victoriametrics.com/blog/proxmox-monitoring-with-dbaas/).