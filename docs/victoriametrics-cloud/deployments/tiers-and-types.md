---
weight: 1
title: "Tiers and Deployment Types"
menu:
  docs:
    parent: "deployments"
    weight: 8
    name: "Tiers and Deployment Types"
tags:
  - metrics
  - cloud
  - enterprise
aliases:
  - /victoriametrics-cloud/tiers-parameters/index.html
---

VictoriaMetrics Cloud offers two different deployment types: **Single-node** and **Cluster**. Both deployment types are based on the VictoriaMetrics [Open Source project](https://github.com/VictoriaMetrics/VictoriaMetrics/),
and managed by the VictoriaMetrics team.

## Single or Cluster?

The first choice for users when creating a VictoriaMetrics deployment is to select a Single-node or Cluster deployment type.

In a nutshell, [Single-node deployments](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) are useful for affordable and performant instances,
while [Cluster deployments](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) are the ideal choice for those use cases that require high availability and multi-tenancy at scale.
More detailed information about the general capabilities of both tiers can be found in this [FAQ](https://docs.victoriametrics.com/victoriametrics/faq/#which-victoriametrics-type-is-recommended-for-use-in-production---single-node-or-cluster).

More in detail, the following topics should be considered when selecting a deployment type:

{{% collapse name="Reliability/SLA" %}}

Both instance types are highly reliable, with SLAs of 99.5% for `Single-node` deployments and 99.9%
for `Cluster` deployments.

{{% /collapse %}}

{{% collapse name="High Availability" %}}

Since `Single-node` deployments are just one instance, they cannot be highly available. In practice,
this means that during configuration changes and software upgrades, your deployment will experience
a few minutes downtime. (This period of unavailability is not included in the SLA).

On the other hand, `Cluster` deployments do not experience such downtimes.

{{% /collapse %}}

{{% collapse name="Multitenancy" %}}

While [Multitenancy](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy)
is supported in the `Cluster` version of VictoriaMetrics Cloud, it is not supported in `Single-node`
instances.

{{% /collapse %}}

{{% collapse name="Scalability" %}}

Internally, `Single-node` deployments may be scaled vertically and `Cluster` deployments horizontally.

In practice, for VictoriaMetrics Cloud tiers, this means that vertical scaling will affect by
constraining some parameters such as the maximum storage size, but horizontal scaling has no such
limitations.

{{% /collapse %}}

{{% collapse name="Data Replication" %}}

Data replication is provided for `Cluster` deployments only. `Single-node` deployments do not have
such capabilities.

{{% /collapse %}}

{{% collapse name="Enterprise features" %}}

[Enterprise features](http://docs.victoriametrics.com/victoriametrics/enterprise/#victoriametrics-enterprise-features)
are available in both `Single-node` and `Cluster` versions. Some of them may take a while to be exposed
in VictoriaMetrics Cloud. If you are missing any feature, or have any request don't hesitate to
contact us at contact us at support-cloud@victoriametrics.com.

{{% /collapse %}}

{{% collapse name="Efficiency and performance" %}}

Both `Single-node` and `Cluster` versions are highly valued for their performance in various benchmarks
and use cases in the industry. Feel free to read more about use cases and articles [here](http://docs.victoriametrics.com/victoriametrics/articles/).

{{% /collapse %}}


## VictoriaMetrics Cloud Parameters: Selecting a Tier

The next important step when deploying a VictoriaMetrics Cloud instance is to select a `Tier`.
Tiers in VictoriaMetrics Cloud are specific presets of `Single-node` or `Cluster` installations
of different sizes, that are derived from testing typical monitoring environments.

> [!IMPORTANT] In summary, you just need to pick the tier that is able to cope with your load.

In this way, we ensure that tiers are optimized for common use cases, and translated into real-world
data (i.e. _parameters_) such as: Ingestion rate, Active Time Series or Read rate.

### Tier selection Parameters

The following parameters are presented to the user when selecting a tier:

| **Parameter**                             | **Maximum Value**                 | **Description**                                                                                                                                                                                                                                 |
|-------------------------------------------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Data Ingestion Rate**                   | Per Tier Limits                   | Number of [time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series) ingested per second.                                                                                                                                         |
| **Active Time Series Count**              | Per Tier Limits                   | Number of [active time series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series) that received at least one data point in the last hour.                                                                                     |
| **Read Rate**                             | Per Tier Limits                   | Number of datapoints retrieved from the database per second.                                                                                                                                                                                    |

<br></br>
Every deployment (Single-Node or Cluster) listed indicates the maximum expected load in Ingestion Rate, Active Time Series and Read Rate.

### Other limits in tiers

The previous simplified list is made upon several tests and assumptions that cover many general use
cases, that lead to establishing other limits that users regularly don't need to take into account
when selecting a tier.

For example, we assume that the Churn Rate is lower than **30%**. You may need to choose a more extensive
deployment for higher Churn Rates, or when combined with a high amount of series being read per query.

Current usage and limits can be checked in the `Monitor` tab of the [deployments](https://console.victoriametrics.cloud/deployments)
section per instance.

A comprehensive list of these parameters is presented here:

| **Parameter**                             | **Maximum Value**                 | **Description**                                                                                                                                                                                                                                 |
|-------------------------------------------|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **New Series Over 24 Hours** (churn rate) | `<= 30% Active Time Series Count`     | Number of new series created in 24 hours. High [churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate) leads to higher resource consumption.                                                                                |
| **Concurrent Requests per Token**         | `<= 600`                          | Maximum concurrent requests per [access token](deployments/access-tokens.md). It is recommended to create separate tokens for different users and environments. This can be adjusted via [support](mailto:support-cloud@victoriametrics.com). |


<br></br>
For a detailed explanation of each parameter, visit the guide on [Understanding Your Setup Size](https://docs.victoriametrics.com/guides/understand-your-setup-size.html).

{{% collapse name="Selecting a Tier: Real-world example" %}}

For a **C.SMALL.HA** `Tier`, you'll find that it's able to process:
- ~100k samples/s Ingestion Rate
- ~2.5M of Active Time Series

This means that, with this `Tier` tou can collect metrics from:
- 10x Kubernetes cluster with 50 nodes each - 4200 * 10 * 50 = 2.1M
- 500 node exporters - 0.5M
- With metrics collection interval - 30s

{{% /collapse %}}

## Selecting Retention and Storage

The last parameter needed to set up a deployment is the Storage needed for this deployment. Recommended
storage is calculated upon **ingestion rate** and desired **retention**.

Keeping in mind that storage can always be increased (but not downsized) **users are recommended to start
small and scale as needed**.

> [!TIP] Flexible storage helps to reduce costs and adapt it to your needs.

For example, the full amount of storage needed for 6 months retention for a given tier will only be
reached after those 6 months of operations. There's no need to reserve storage from the beginning.

Features like Downsampling, Data Deduplication, Cardinality Explorer or Metrics usage are encouraged to
further reduce your costs. Feel free to contact [support](mailto:support-cloud@victoriametrics.com) if
you need more information.

## Advanced Parameters: Flags

Additionally, VictoriaMetrics Cloud exposes certain parameters (or [command-line flags](https://docs.victoriametrics.com/#list-of-command-line-flags))
that **advanced users** can tweak on their own under the `Advanced settings` section of every deployment
after creation.

> [!WARNING] Changing default command-line flags may lead to errors
> Modifying Advanced parameters can result into changes in resource consumption usage, causing a
> deployment not being able to compute the load they were designed to support. In these cases,
> a higher tier is most probably needed.

Some of these advanced parameters are outlined below:

| **Flag**                               | **Description**                                                             |
|----------------------------------------|-----------------------------------------------------------------------------|
| <nobr>`-maxLabelsPerTimeseries`</nobr> | Maximum number of labels per time series. Time series with excess labels are dropped. Higher values can increase [cardinality](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#cardinality) and resource usage.  |
| `-maxLabelValueLen`                    | Maximum length of label values. Time series with longer values are dropped. Large label values can lead to high RAM consumption. This parameter is not exposed and can only be adjusted via [support](mailto:support-cloud@victoriametrics.com). **In general, label values with high values `~>1kb` are not supported**. |

## Terms and definitions

  - [Time series](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#time-series)
  - [Labels](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels)
  - [Active time series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series)
  - [Churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate)
  - [Cardinality](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#cardinality)
