In the dynamic and complex world of system monitoring, [VictoriaMetrics Anomaly Detection](https://victoriametrics.com/products/enterprise/anomaly-detection/) (or shortly, `vmanomaly`), being a part of our [Enterprise offering](https://victoriametrics.com/products/enterprise/), stands as a pivotal tool for achieving advanced observability. It empowers SREs and DevOps teams by automating the identification of abnormal behavior in time-series data. It goes beyond traditional threshold-based alerting, utilizing machine learning techniques to not only detect anomalies but also minimize false positives, thus reducing alert fatigue. By providing simplified alerting mechanisms atop of [unified anomaly scores](https://docs.victoriametrics.com/anomaly-detection/components/models/#vmanomaly-output), it enables teams to spot and address potential issues faster, ensuring system reliability and operational efficiency.

## What does it do?
- Designed to periodically scan new data points across selected metrics, it forecasts unified [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score).
- Scores are recorded back to VictoriaMetrics TSDB for utilization in subsequent applications, such as alerting services.
- Simplified alerting rules can be established and observability insights received, enhancing your operational efficiency.

## How does it work?
At its core, VictoriaMetrics Anomaly Detection autonomously re-trains either pre-defined machine learning models or custom models tailored to your business needs on your data.

- ML models are employed to calculate anomaly scores for newly collected data points, as per a predefined schedule.
- Alerts can be triggered based on simplified thresholds (i.e. anomaly_score > 1) that simplify and automate your observability setup.
- Ongoing evaluations, presented either as specific point estimates or as ranges of confidence intervals, are designed to integrate seamlessly with downstream applications.

## Practical Guides and Installation

Get started with VictoriaMetrics Anomaly Detection efficiently by following our guides and installation options:

- **Quickstart**: Learn how to quickly set up `vmanomaly` by following the [Quickstart Guide](https://docs.victoriametrics.com/anomaly-detection/quickstart/).
- **Integration**: Integrate anomaly detection into your existing observability stack. Find detailed steps [here](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/).
- **Anomaly Detection Presets**: Enable anomaly detection on predefined sets of metrics that require frequent static threshold changes for alerting. Learn more [here](https://docs.victoriametrics.com/anomaly-detection/presets/).

- **Installation Options**: Choose the installation method that best fits your infrastructure:
    - **Docker Installation**: Ideal for containerized environments. Follow the [Docker Installation Guide](https://docs.victoriametrics.com/anomaly-detection/quickstart/#docker).
    - **Helm Chart Installation**: Recommended for Kubernetes deployments. See our [Helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).

- **Self-Monitoring**: Ensure `vmanomaly` is functioning optimally with built-in self-monitoring capabilities. Use the provided Grafana dashboards and alerting rules to track service health and operational metrics. Find the complete docs [here](https://docs.victoriametrics.com/anomaly-detection/self-monitoring/).

> **Note**: starting from [v1.5.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v150) `vmanomaly` requires a [license key](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing) to run. You can obtain a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

## Key Components
Explore the integral components that configure VictoriaMetrics Anomaly Detection:
* [Explore components and their interation](https://docs.victoriametrics.com/anomaly-detection/components/)
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
