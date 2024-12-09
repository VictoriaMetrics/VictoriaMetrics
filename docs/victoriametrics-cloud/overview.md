---
weight: 1
title: Overview of VictoriaMetrics Cloud 
menu:
  docs:
    parent: "cloud"
    weight: 1
    name: Overview
aliases:
  - /victoriametrics-cloud/overview/index.html
  - /managed-victoriametrics/overview/index.html
---
VictoriaMetrics is a fast and easy-to-use monitoring solution and time series database. 
It integrates well with existing monitoring systems such as Grafana, Prometheus, Graphite, 
InfluxDB, OpenTSDB and DataDog - see [these docs](https://docs.victoriametrics.com/#how-to-import-time-series-data) for details. 

The most common use cases for VictoriaMetrics are:
* Long-term remote storage for Prometheus;
* More efficient drop-in replacement for Prometheus and Graphite
* Replacement for InfluxDB and OpenTSDB, which uses lower amounts of RAM, CPU and disk;
* Cost-efficient alternative for DataDog.

VictoriaMetrics Cloud allows users to run Enterprise version of VictoriaMetrics, hosted on AWS, without the need to perform typical 
DevOps tasks such as proper configuration, monitoring, logs collection, access protection, software updates, 
backups, etc. [Try it right now](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_overview)

We run VictoriaMetrics Cloud deployments in our environment on AWS and provide easy-to-use endpoints 
for data ingestion and querying. The VictoriaMetrics team takes care of optimal configuration and software 
maintenance.

VictoriaMetrics Cloud comes with the following features:

* It can be used as a Managed Prometheus - just configure Prometheus or vmagent to write data to VictoriaMetrics Cloud and then use the provided endpoint as a Prometheus datasource in Grafana;
* Built-in [Alerting & Recording](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/#configure-alerting-rules) rules execution;
* Hosted [Alertmanager](https://docs.victoriametrics.com/victoriametrics-cloud/alertmanager-setup-for-deployment/) for sending notifications;
* Every VictoriaMetrics Cloud deployment runs in an isolated environment, so deployments cannot interfere with each other;
* VictoriaMetrics Cloud deployment can be scaled up or scaled down in a few clicks;
* Automated backups;
* Pay only for the actually used resources - compute, storage, traffic.
