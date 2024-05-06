---
sort: 1
weight: 1
title: Presets
menu:
  docs:
    parent: "anomaly-detection"
    weight: 1
    title: Presets
---
# Anomaly Detection Presets
> Please check the [Quick Start Guide](/anomaly-detection/quickstart/) to install and run `vmanomaly`

> Presets are available from v1.13.0

Presets enable anomaly detection in indicators that are hard to monitor using alerts based on static thresholds.
So, the anomaly detection alerting rules based on the [`anomaly_scores`](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) stay the same over time, and we generate the anomaly scores using predefined machine learning models.
Models are constantly retraining on different time frames which helps to keep alerts up to date and to consider underlying data patterns.

You can set up the simplified configuration file for `vmanomaly` just specifying the type of preset and data sources in [`reader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/) and [`writer`](https://docs.victoriametrics.com/anomaly-detection/components/writer/) sections of the config.
The rest of the parameters are already set up for you.

Available presets:
- [Node-Exporter](/anomaly-detection/presets/node-exporter.html)

Here is an example config file to enable Node-Exporter preset:

```yaml
preset: "node-exporter"
reader:
  datasource_url: "http://victoriametrics:8428/" # your datasource url
  # tenant_id: '0:0'  # specify for cluster version
writer:
  datasource_url: "http://victoriametrics:8428/" # your datasource url
  # tenant_id: '0:0'  # specify for cluster version
```
Run a service using config file with one of the [available options](/anomaly-detection/quickstart/#how-to-install-and-run-vmanomaly).

After you run `vmanomaly`, the available assets can be found here: `http://localhost:8490/presets/`

<img alt="preset-localhost" src="vmanomaly-preset-localhost.webp">

