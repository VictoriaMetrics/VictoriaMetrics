---
sort: 2
weight: 4
title: FAQ
menu:
  docs:
    identifier: "vmanomaly-faq"
    parent: "anomaly-detection"
    weight: 4
aliases:
- /anomaly-detection/FAQ.html
---

# FAQ - VictoriaMetrics Anomaly Detection

## What is VictoriaMetrics Anomaly Detection (vmanomaly)?
VictoriaMetrics Anomaly Detection, also known as `vmanomaly`, is a service for detecting unexpected changes in time series data. Utilizing machine learning models, it computes and pushes back an ["anomaly score"](/anomaly-detection/components/models.html#vmanomaly-output) for user-specified metrics. This hands-off approach to anomaly detection reduces the need for manual alert setup and can adapt to various metrics, improving your observability experience.

Please refer to [our guide section](/anomaly-detection/#practical-guides-and-installation) to find out more.

> **Note: `vmanomaly` is a part of [enterprise package](https://docs.victoriametrics.com/enterprise.html). You need to get a [free trial license](https://victoriametrics.com/products/enterprise/trial/) for evaluation.**

## What is anomaly score?
Among the metrics produced by `vmanomaly` (as detailed in [vmanomaly output metrics](/anomaly-detection/components/models.html#vmanomaly-output)), `anomaly_score` is a pivotal one. It is **a continuous score > 0**, calculated in such a way that **scores ranging from 0.0 to 1.0 usually represent normal data**, while **scores exceeding 1.0 are typically classified as anomalous**. However, it's important to note that the threshold for anomaly detection can be customized in the alert configuration settings.

The decision to set the changepoint at `1.0` is made to ensure consistency across various models and alerting configurations, such that a score above `1.0` consistently signifies an anomaly, thus, alerting rules are maintained more easily.

> Note: `anomaly_score` is a metric itself, which preserves all labels found in input data and (optionally) appends [custom labels, specified in writer](/anomaly-detection/components/writer.html#metrics-formatting) - follow the link for detailed output example.

## How does vmanomaly work?
`vmanomaly` applies built-in (or custom) [anomaly detection algorithms](/anomaly-detection/components/models.html), specified in a config file. Although a single config file supports one model, running multiple instances of `vmanomaly` with different configs is possible and encouraged for parallel processing or better support for your use case (i.e. simpler model for simple metrics, more sophisticated one for metrics with trends and seasonalities).

1. For more detailed information, please visit the [overview section](/anomaly-detection/Overview.html#about).
2. To view a diagram illustrating the interaction of components, please explore the [components section](/anomaly-detection/components/).

## What data does vmanomaly operate on?
`vmanomaly` operates on data fetched from VictoriaMetrics, where you can leverage full power of [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) for data selection, sampling, and processing. Users can also [apply global filters](https://docs.victoriametrics.com/#prometheus-querying-api-enhancements) for more targeted data analysis, enhancing scope limitation and tenant visibility.

Respective config is defined in a [`reader`](/anomaly-detection/components/reader.html#vm-reader) section.

## Handling noisy input data
`vmanomaly` operates on data fetched from VictoriaMetrics using [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) queries, so the initial data quality can be fine-tuned with aggregation, grouping, and filtering to reduce noise and improve anomaly detection accuracy.

## Output produced by vmanomaly
`vmanomaly` models generate [metrics](/anomaly-detection/components/models.html#vmanomaly-output) like `anomaly_score`, `yhat`, `yhat_lower`, `yhat_upper`, and `y`. These metrics provide a comprehensive view of the detected anomalies. The service also produces [health check metrics](/anomaly-detection/components/monitoring.html#metrics-generated-by-vmanomaly) for monitoring its performance.

## Choosing the right model for vmanomaly
Selecting the best model for `vmanomaly` depends on the data's nature and the types of anomalies to detect. For instance, [Z-score](anomaly-detection/components/models.html#z-score) is suitable for data without trends or seasonality, while more complex patterns might require models like [Prophet](anomaly-detection/components/models.html#prophet).

Please refer to [respective blogpost on anomaly types and alerting heuristics](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/) for more details.

Still not 100% sure what to use? We are [here to help](/anomaly-detection/#get-in-touch).

## Alert generation in vmanomaly
While `vmanomaly` detects anomalies and produces scores, it *does not directly generate alerts*. The anomaly scores are written back to VictoriaMetrics, where an external alerting tool, like [`vmalert`](/vmalert.html), can be used to create alerts based on these scores for integrating it with your alerting management system.

## Preventing alert fatigue
Produced anomaly scores are designed in such a way that values from 0.0 to 1.0 indicate non-anomalous data, while a value greater than 1.0 is generally classified as an anomaly. However, there are no perfect models for anomaly detection, that's why reasonable defaults expressions like `anomaly_score > 1` may not work 100% of the time. However, anomaly scores, produced by `vmanomaly` are written back as metrics to VictoriaMetrics, where tools like [`vmalert`](/vmalert.html) can use [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) expressions to fine-tune alerting thresholds and conditions, balancing between avoiding [false negatives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-negative) and reducing [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive).

## Resource consumption of vmanomaly
`vmanomaly` itself is a lightweight service, resource usage is primarily dependent on [scheduling](/anomaly-detection/components/scheduler.html) (how often and on what data to fit/infer your models), [# and size of timeseries returned by your queries](/anomaly-detection/components/reader.html#vm-reader), and the complexity of the employed [models](anomaly-detection/components/models.html). Its resource usage is directly related to these factors, making it adaptable to various operational scales.

## Scaling vmanomaly
`vmanomaly` can be scaled horizontally by launching multiple independent instances, each with its own [MetricsQL](https://docs.victoriametrics.com/MetricsQL.html) queries and [configurations](/anomaly-detection/components/). This flexibility allows it to handle varying data volumes and throughput demands efficiently.