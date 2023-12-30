---
sort: 14
title: Anomaly Detection
weight: 7
disableToc: true

menu:
  docs:
    parent: 'victoriametrics'
    weight: 7

aliases:
- /anomaly-detection.html
---

# VictoriaMetrics Anomaly Detection

In the dynamic and complex world of system monitoring, VictoriaMetrics Anomaly Detection, being a part of our [Enterprise offering](https://victoriametrics.com/products/enterprise/), stands as a pivotal tool for achieving advanced observability. It empowers SREs and DevOps teams by automating the intricate task of identifying abnormal behavior in time-series data. This powerful feature goes beyond traditional threshold-based alerting, utilizing sophisticated machine learning techniques to not only detect anomalies but also minimize false positives, thus reducing alert fatigue. By providing simplified alerting mechanisms atop of [unified anomaly scores](/anomaly-detection/docs/models/models.html#vmanomaly-output), it enables teams to spot and address potential issues faster, ensuring system reliability and operational efficiency.

## Key Components and Documentation
Explore the integral components that configure VictoriaMetrics Anomaly Detection:
* [Get familiar with components](/anomaly-detection/docs)
    - [Models](/anomaly-detection/docs/models)
    - [Monitoring](/anomaly-detection/docs/monitoring.html)
    - [Reader](/anomaly-detection/docs/reader.html)
    - [Scheduler](/anomaly-detection/docs/scheduler.html)
    - [Writer](/anomaly-detection/docs/writer.html)

## Practical Guides and Installation
Kickstart your journey with our guides and installation instructions:
* [Quick Start](/anomaly-detection/guides/guide-vmanomaly-vmalert.html) - Set up anomaly detection and alerting
* Installation Methods
    - [Docker guide](../vmanomaly.md#run-vmanomaly-docker-container)
    - [Helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly)

## Deep Dive into Anomaly Detection
Enhance your knowledge with our handbook on Anomaly Detection & Root Cause Analysis and stay updated:
* Anomaly Detection Handbook
    - [Introduction to Time Series Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/)
    - [Types of Anomalies in Time Series Data](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/)
    - [Techniques and Models for Anomaly Detection](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-3/)
* Follow the [`#anomaly-detection`](https://victoriametrics.com/blog/tags/anomaly-detection/) tag in our blog

## Engage with VictoriaMetrics
Connect with us for a personalized experience:
* [Book a Demo](https://calendly.com/fred-navruzov/)
* [Request Trial Enterprise License](https://new.victoriametrics.com/products/enterprise/trial/), including Anomaly Detection

---
Please find [CHANGELOG here](./CHANGELOG.md)