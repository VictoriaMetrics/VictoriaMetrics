---
weight: 5
title: "VictoriaMetrics Cloud: Tier Parameters and Flag Parameters Configuration"
menu:
  docs:
    parent: "cloud"
    weight: 8
    name: "Tier Parameters and Flag Parameters Configuration"
---

The tier parameters are derived from testing in typical monitoring environments, ensuring they are optimized for common use cases.

## VictoriaMetrics Cloud Tier Parameters

| **Parameter**                             | **Maximum Value**                 | **Description**                                                                                                                                                                                    |
|-------------------------------------------|-----------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Data Ingestion Rate**                   | Per Tier Limits                   | Number of [time series](https://docs.victoriametrics.com/keyconcepts/#time-series) ingested per second.                                                                                                                                   |
| **Active Time Series Count**              | Per Tier Limits                   | Number of [active time series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series) that received at least one data point in the last hour.                                         |
| **Read Rate**                             | Per Tier Limits                   | Number of datapoints retrieved from the database per second.                                                                                                                                       |
| **New Series Over 24 Hours** (churn rate) | `<= Active Time Series Count`     | Number of new series created in 24 hours. High [churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate) leads to higher resource consumption.                                    |
| **Concurrent Requests per Token**         | `<= 600`                          | Maximum concurrent requests per access token. It is recommended to create separate tokens for different clients and environments. This can be adjusted via [support](mailto:support@victoriametrics.com). |

For a detailed explanation of each parameter, visit the guide on [Understanding Your Setup Size](https://docs.victoriametrics.com/guides/understand-your-setup-size.html).

## Flag Parameters Configuration

| **Flag**                          | **Default Value**         | **Description**                                                                                                                                                                                                                                                                                                                                    |
|-----------------------------------|---------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Max Label Value Length**        | `<= 1kb` (Default: `4kb`) | Maximum length of label values. Longer values are truncated. Large label values can lead to high RAM consumption. This can be adjusted via [support](mailto:support@victoriametrics.com).                                                                                                                                                          |
| **Max Labels per Time Series**    | `<= 30`                   | Maximum number of labels per time series. Excess labels are dropped. Higher values can increase [cardinality](https://docs.victoriametrics.com/keyconcepts/#cardinality) and resource usage. This can be configured in [deployment settings](https://docs.victoriametrics.com/victoriametrics-cloud/quickstart/#modifying-an-existing-deployment). |


## Terms and definitions:

  - [Time series](https://docs.victoriametrics.com/keyconcepts/#time-series)
  - [Labels](https://docs.victoriametrics.com/keyconcepts/#labels)
  - [Active time series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series)
  - [Churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate)
  - [Cardinality](https://docs.victoriametrics.com/keyconcepts/#cardinality)
