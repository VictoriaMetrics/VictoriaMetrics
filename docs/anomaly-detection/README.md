---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

In today's fast-paced and complex landscape of system monitoring, [VictoriaMetrics Anomaly Detection](https://victoriametrics.com/products/enterprise/anomaly-detection/) (`vmanomaly`), a part of our [Enterprise offering](https://victoriametrics.com/products/enterprise/), serves as an **observability layer** for SREs and DevOps teams atop of collected data to **automate the detection of anomalies in time-series data**, reducing manual efforts required to identify abnormal system behavior.

Unlike traditional threshold-based alerting, which relies on **raw metric values** and requires constant tuning and maintenance of thresholds and alerting rules, `vmanomaly` introduces a **unified, interpretable [anomaly score](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score)** - a **de-trended, de-seasonalized metric** generated through machine learning. This approach eliminates the need for frequent manual adjustments by enabling **stable, long-term static thresholds (as simple as `anomaly_score > 1`)** that remain effective over time through continuous model retraining.

By shifting to anomaly-based detection, teams can **identify and respond to potential issues faster**, enhancing system reliability and operational efficiency while significantly **reducing the engineering effort spent on handcrafting and maintaining alerting rules**.


## What does it do?

`vmanomaly` is designed to **periodically analyze new data points** across selected metrics (either requested from [VictoriaMetrics TSDB](https://docs.victoriametrics.com/victoriametrics/) or produced by [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) metrics [endpoint](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats)), generating a **unified metric** called [anomaly score](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score). 

Key functions:
- **Automated anomaly detection** - continuously scans time-series data to identify deviations from expected behavior.
- **Seamless integration** - anomaly scores are stored in VictoriaMetrics TSDB for use in **alerting, visualization, and downstream analytics**.

The diagram below illustrates how `vmanomaly` fits into an observability setup, such as detecting anomalies in metrics collected by `node_exporter`:

<img src="https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/guide-vmanomaly-vmalert_overview.webp" alt="node_exporter_example_diagram" style="width:60%"/>

## How does it work?

VictoriaMetrics Anomaly Detection **continuously re-fit and apply machine learning models** - either [built-in](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models) or [custom](https://docs.victoriametrics.com/anomaly-detection/components/models/#custom-model-guide), specific to your business needs — on your [input](https://docs.victoriametrics.com/anomaly-detection/components/reader/) data. This ensures that the default cut-off threshold (`anomaly score == 1`), which differentiates **normal** (`≤ 1`) from **anomalous** (`> 1`) data points, remains **relevant over time**.

- **Automated anomaly scoring** - ML models calculate [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) for new data points based on a predefined [schedule](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/).
- **Simplified alerting** - alerts can be triggered using **straightforward thresholds** (e.g., `anomaly_score > 1`), reducing complexity in observability setups.
- **Additional model outputs** - beyond anomaly scores, models provide [supplementary outputs](https://docs.victoriametrics.com/anomaly-detection/components/models/#vmanomaly-output), including:
  - **Point estimates** (`yhat`)  
  - **Confidence intervals** (`[yhat_lower, yhat_upper]`)  
  These outputs integrate seamlessly into downstream applications, making it easier to **visually inspect anomalies**, e.g. in respective [Grafana dashboards](https://docs.victoriametrics.com/anomaly-detection/presets/#grafana-dashboard).

<img src="https://docs.victoriametrics.com/anomaly-detection/components/vmanomaly-components.webp" alt="node_exporter_example_diagram" style="width:80%"/>

## Key benefits

`vmanomaly` is designed to **reduce MTTR (Mean Time to Resolution)** in observability workflows by **automating anomaly detection** and **eliminating the need for manual threshold tuning**. It is particularly beneficial for:

- **Reducing alerting rule maintenance** – shifts from manually maintaining static thresholds on raw metric values to a **stable anomaly score threshold** that remains **reliable and interpretable over time**.
  
- **Handling complex metrics** – effectively detects anomalies in **trending, seasonal, or dynamically scaling data**, where **fixed thresholds and simpler models usually fail**.

- **Detecting anomalies in interconnected metrics** – supports **[multivariate anomaly detection](http://docs.victoriametrics.com/anomaly-detection/components/models#multivariate-models)**, identifying patterns across **related metrics** instead of treating them in isolation as [univariate metrics](http://docs.victoriametrics.com/anomaly-detection/components/models#univariate-models).

## Practical guides and installation

Get started with VictoriaMetrics Anomaly Detection by following our guides and installation options:

- **Quickstart**: Learn how to quickly set up `vmanomaly` by following the [Quickstart Guide](https://docs.victoriametrics.com/anomaly-detection/quickstart/).
- **UI**: Explore anomaly detection configurations through the [vmanomaly UI](https://docs.victoriametrics.com/anomaly-detection/ui/).
- **Integration**: Integrate anomaly detection into your existing observability stack. Find detailed steps [here](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/).
- **Anomaly Detection Presets**: Enable anomaly detection on predefined sets of metrics. Learn more [here](https://docs.victoriametrics.com/anomaly-detection/presets/).

- **Installation Options**: Choose the installation method that best fits your infrastructure:
    - **Docker Installation**: Ideal for containerized environments. Follow the [Docker Installation Guide](https://docs.victoriametrics.com/anomaly-detection/quickstart/#docker).
    - **Helm Chart Installation**: Recommended for Kubernetes deployments. See our [Helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).
    - **Kubernetes Custom Resource**: If you are using [VM Operator](https://docs.victoriametrics.com/operator/), deploy `vmanomaly` using the [custom resource guide](https://docs.victoriametrics.com/operator/resources/vmanomaly/).

- **High Availability**: See how to enable [horizontal scalability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability) and [high availability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#high-availability) for `vmanomaly` service [here](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/)

- **Self-Monitoring**: Ensure `vmanomaly` is functioning optimally, using provided Grafana dashboards and alerting rules to track service health and operational metrics. Find the guide [here](https://docs.victoriametrics.com/anomaly-detection/self-monitoring/).

> Starting from [v1.5.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v150) `vmanomaly` requires a [license key](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing) to run. You can obtain a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

## Key Components
Explore the [integral components](https://docs.victoriametrics.com/anomaly-detection/components/) that define VictoriaMetrics Anomaly Detection:
- [Models](https://docs.victoriametrics.com/anomaly-detection/components/models/)
- [Reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/)
- [Scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/)
- [Writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/)
- [Monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/)

## Deep Dive into Anomaly Detection
Enhance your knowledge with our handbook on Anomaly Detection & Root Cause Analysis and stay updated:
* Anomaly Detection Handbook
    - [Introduction to Time Series Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/)
    - [Types of Anomalies in Time Series Data](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/)
    - [Techniques and Models for Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-3/)
* Follow the [`#anomaly-detection`](https://victoriametrics.com/tags/anomaly-detection/) tag in our blog

## Product Updates
Stay up-to-date with the latest improvements and features in VictoriaMetrics Anomaly Detection, and the rest of our products on our [blog](https://victoriametrics.com/tags/product-updates/).

## Frequently Asked Questions (FAQ)
Got questions about VictoriaMetrics Anomaly Detection? Chances are, we've got the answers ready for you. 

Dive into [our FAQ section](https://docs.victoriametrics.com/anomaly-detection/faq/) to find responses to common questions.

## Get in Touch
We are eager to connect with you and adapt our solutions to your specific needs. Here's how you can engage with us:
* [Book a Demo](https://calendly.com/victoriametrics-anomaly-detection) to discover what our product can do.
* Interested in exploring our [Enterprise features](https://victoriametrics.com/products/enterprise), including [Anomaly Detection](https://victoriametrics.com/products/enterprise/anomaly-detection)? [Request your trial license](https://victoriametrics.com/products/enterprise/trial/) today and take the first step towards advanced system observability.

---
Our [CHANGELOG is just a click away](https://docs.victoriametrics.com/anomaly-detection/changelog/), keeping you informed about the latest updates and enhancements.
