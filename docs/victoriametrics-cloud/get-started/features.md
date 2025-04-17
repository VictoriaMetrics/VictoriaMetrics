---
weight: 2
title: Key Features & Benefits
menu:
  docs:
    parent: get-started
    weight: 2
aliases:
  - /victoriametrics-cloud/quickstart/features.html
  - /managed-victoriametrics/quickstart/features.html
---

VictoriaMetrics Cloud helps optimizing your data and maximizing its value in the most reliable way. It can be used as an **Enterprise-level Managed Prometheus**: just configure Prometheus, [vmagent](https://docs.victoriametrics.com/vmagent/), an OpenTelemetry Collector or any agent to write data to VictoriaMetrics Cloud, and point Grafana to VictoriaMetrics Cloud by configuring it as a Prometheus datasource.

## Features
VictoriaMetrics Cloud offers a robust suite of features designed to optimize your cloud experience. Seamless integrations, scalability and cost-saving measures, and comprehensive operational tools ensure that VictoriaMetrics Cloud can support your business needs.

{{% collapse name="Integrations and Compatibility" %}}

* **Observability protocols**: Prometheus, OpenTelemetry, InfluxDB, DataDog, NewRelic, OpenTSDB & Graphite.
* **Data visualization**: Use built-in [VictoriaMetrics UI](https://play.victoriametrics.com/) or integrate seamlessly with your current stack to query and visualize your data in [Grafana](https://grafana.com/) or [Perses](https://perses.dev).
* [**AWS PrivateLink**](https://aws.amazon.com/privatelink/): enabling even more secure communication with VictoriaMetrics Cloud deployments directly from your VPC.

![Integrations](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/features_integrations.webp)
<figcaption style="text-align: center; font-style: italic;">VictoriaMetrics Cloud Integrations</figcaption>

{{% /collapse %}}

{{% collapse name="Scale as you go and save costs" %}}

* **Easy Scaling**: VictoriaMetrics Cloud deployments can be scaled up or down with just a few clicks in line with growth and needs.
* **Downsampling**: Lower your disk footprint (and save on storage costs!) by keeping fewer data points for historical data and speed up queries for it, while preserving high precision for your operational data.
* **Retention filters**: Configure a custom retention period on a team (tenant) level or time series level by using label filters so that unneeded time series are wiped out freeing up storage space for new metrics data enabling additional cost savings
* **Recording rules**: Improve query performance with recording rules, facilitating quicker data access & dashboard responsiveness.

{{% /collapse %}}

{{% collapse name="Operations" %}}

* **Enterprise, managed VictoriaMetrics Solution**: Comes with all the proven features in VictoriaMetrics open source & Enterprise.
* **Single-node** & **Cluster** configurations with automatic software version and security updates.
* Built-in [Alerting & Recording](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/#configure-alerting-rules) rules execution. Define your rules & get immediate alerts as issues arise, enabling swift action & minimizing disruption to your users.
* Hosted [Alertmanager](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/) for sending notifications.
* **Isolated Deployments**: VictoriaMetrics Cloud provisions dedicated resources for your deployments, so you won’t encounter “noisy neighbors” problems as deployments do not compete for resources.
* **Multitenancy**: Easily serve multiple teams (tenants) with one Cluster deployment by having a dedicated namespace for each team.
* **Automated Backups**: Regular backup procedures are in place. Your data is automatically saved to a backup storage, so you can easily restore it when the need arises.
* **High-availability** & replication.
* **Reliability** & extraordinary performance with 99.95% SLA.

{{% /collapse %}}

## Get instant value from your data

VictoriaMetrics Cloud allows you to explore and optimize both your data and deployments.

{{% collapse name="Query your own metrics" %}}

* Visualize your own data in graphs, table or json formats
* Combine several queries at the same time
* Prettify your queries to improve readability
* Autocomplete to help you writing queries
* Trace your queries to understand behavior

![Query](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/features_query.webp)
<figcaption style="text-align: center; font-style: italic;">Query your data with VictoriaMetrics Cloud</figcaption>

{{% /collapse %}}

{{% collapse name="Explore valuable insights" %}}

* List your Prometheus metrics by Job and Instance
* Inspect your time series data cardinality to optimize usage and costs
* Discover top used or heaviest queries

![Cardinality](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/features_cardinality.webp)
<figcaption style="text-align: center; font-style: italic;">Understand your data with VictoriaMetrics Cloud</figcaption>

{{% /collapse %}}

{{% collapse name="Analyze, debug and learn" %}}

* Trace and query analyzer to debug queries
* WITH templating for MetricsQL: functions, variables and filters
* Debug metrics relabling with easy-to-follow examples

![Traces](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/features_traces.webp)
<figcaption style="text-align: center; font-style: italic;">Debug your queries</figcaption>

{{% /collapse %}}

## Benefits
In brief, we run VictoriaMetrics Cloud deployments in our AWS environment and provide direct endpoints
for data ingestion and querying. The VictoriaMetrics team takes care of optimal configuration and software
maintenance. You can think of it as having access to a **fully supported, enterprise** version of VictoriaMetrics
that runs outside your environment, helping you to save resources and costs, without the hustle of performing
typical DevOps tasks such as configuration management, monitoring, log collection, access protection, perform
software and infrastructure upgrades, store backups regularly or control costs. **We take care of that**.

> VictoriaMetrics Cloud is able to handle larger workloads than competing solutions at a far lower cost.

{{% collapse name="Easy Migration" %}}

* Migrate from costly & less scalable monitoring solutions such as Managed Prometheus service from AWS, GCP or Azure, InfluxDB Cloud, or your on-premises setup.
* Get higher data resolution with much higher cardinality.
* Run more complex queries.

{{% /collapse %}}

{{% collapse name="Enterprise level support" %}}

Includes all VictoriaMetrics Enterprise Features Plus:

* Business days & hours support
* 8 hours response time for system impaired issues

{{% /collapse %}}

{{% collapse name="Cost-efficient Scaling" %}}

* Only pay for the resources that you actually use (compute, disk and network).
* Downsampling and retention filters features enable additional cost-savings.

{{% /collapse %}}

{{% collapse name="Ease of Budgeting" %}}

**No invoice surprises**: pick a tier at a fixed price. Our pricing model protects you from surprise overages coming from unexpected changes in workload such as spikes in data ingestion rate, cardinality explosions or accidental heavy queries.

{{% /collapse %}}


{{% collapse name="Ease of use" %}}

The VictoriaMetrics team takes care of optimal configuration and handles all software maintenance, so you can focus on the monitoring.

{{% /collapse %}}

