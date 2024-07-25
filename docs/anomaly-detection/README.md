In the dynamic and complex world of system monitoring, VictoriaMetrics Anomaly Detection, being a part of our [Enterprise offering](https://victoriametrics.com/products/enterprise/), stands as a pivotal tool for achieving advanced observability. It empowers SREs and DevOps teams by automating the intricate task of identifying abnormal behavior in time-series data. It goes beyond traditional threshold-based alerting, utilizing machine learning techniques to not only detect anomalies but also minimize false positives, thus reducing alert fatigue. By providing simplified alerting mechanisms atop of [unified anomaly scores](./components/models.md#vmanomaly-output), it enables teams to spot and address potential issues faster, ensuring system reliability and operational efficiency.

## Practical Guides and Installation
Begin your VictoriaMetrics Anomaly Detection journey with ease using our guides and installation instructions:

- **Quickstart**: Check out how to get `vmanomaly` up and running [here](./QuickStart.md).
- **Overview**: Find out how `vmanomaly` service operates [here](./Overview.md)
- **Integration**: Integrate anomaly detection into your observability ecosystem. Get started [here](./guides/guide-vmanomaly-vmalert/README.md).
- **Anomaly Detection Presets**: Enable anomaly detection on predefined set of indicators, that require frequently changing static thresholds for alerting. Find more information [here](./Presets.md).

- **Installation Options**: Select the method that aligns with your technical requirements:
    - **Docker Installation**: Suitable for containerized environments. See [Docker guide](./Overview.md#run-vmanomaly-docker-container).
    - **Helm Chart Installation**: Appropriate for those using Kubernetes. See our [Helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).


> **Note**: starting from [v1.5.0](./CHANGELOG.md#v150) `vmanomaly` requires a [license key](./Overview.md#licensing) to run. You can obtain a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

## Key Components
Explore the integral components that configure VictoriaMetrics Anomaly Detection:
* [Explore components and their interation](./components/README.md)
    - [Models](./components/models.md)
    - [Reader](./components/reader.md)
    - [Scheduler](./components/scheduler.md)
    - [Writer](./components/writer.md)
    - [Monitoring](./components/monitoring.md)

## Deep Dive into Anomaly Detection
Enhance your knowledge with our handbook on Anomaly Detection & Root Cause Analysis and stay updated:
* Anomaly Detection Handbook
    - [Introduction to Time Series Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/)
    - [Types of Anomalies in Time Series Data](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/)
    - [Techniques and Models for Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-3/)
* Follow the [`#anomaly-detection`](https://victoriametrics.com/blog/tags/anomaly-detection/) tag in our blog

## Frequently Asked Questions (FAQ)
Got questions about VictoriaMetrics Anomaly Detection? Chances are, we've got the answers ready for you. 

Dive into [our FAQ section](./FAQ.md) to find responses to common questions.

## Get in Touch
We are eager to connect with you and adapt our solutions to your specific needs. Here's how you can engage with us:
* [Book a Demo](https://calendly.com/victoriametrics-anomaly-detection) to discover what our product can do.
* Interested in exploring our [Enterprise features](https://victoriametrics.com/products/enterprise), including [Anomaly Detection](https://victoriametrics.com/products/enterprise/anomaly-detection)? [Request your trial license](https://victoriametrics.com/products/enterprise/trial/) today and take the first step towards advanced system observability.

---
Our [CHANGELOG is just a click away](./CHANGELOG.md), keeping you informed about the latest updates and enhancements.
