---
weight: 13
title: FAQ about VictoriaMetrics Cloud
disableToc: true
menu:
  docs:
    parent: "cloud"
    weight: 13
    name: FAQ VictoriaMetrics Cloud
---

## What authentication and authorization mechanisms does VictoriaMetrics Cloud support?

* Console (UI) login options can be found in the [Registration and trial](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/registration-and-trial/) section.
* To interact programmatically with VictoriaMetrics Cloud deployments (sending or querying data), bearer tokens are used. See an example in [Quick start](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/quickstart/#vmagent) or tailored examples under the [Integrations](https://cloud.victoriametrics.com/integrations) section.
* To perform console API operations (automated actions with deployments, access tokens, alerting/recording rules), [API Keys](https://docs.victoriametrics.com/victoriametrics-cloud/api/) are used.

Our roadmap is always evolving, so feel free to let us know any requirements you may have at support-cloud@victoriametrics.com.

## What permissions does VictoriaMetrics Cloud require on my AWS resources?
VictoriaMetrics Cloud doesn’t require any permissions. Victoria Metrics Cloud instances are not deployed in your environment, but in a separate one. Interactions are made via https.

## How does VictoriaMetrics Cloud handle data encryption?
Information exchange is secured with TLS.

## Does VictoriaMetrics Cloud require public internet access to AWS services, or can it integrate via AWS Private Link or VPC peering?
Victoria Metrics Cloud doesn’t require access to AWS services, quite the opposite: your services need access to VictoriaMetrics Cloud for writing and querying metrics or logs. The only scenario where a call from the platform to your services can occur is when sending notifications about alerts (if you configure the notifications service to be running in your environment).

It's not mandatory to use public internet access, the option to use AWS Private Link is available, at an extra cost derived from direct AWS cost of the service.

## What networking requirements does VictoriaMetrics Cloud have (IP whitelisting, VPN, Direct Connect, etc.)?
None.

## Does VictoriaMetrics Cloud support VPC endpoints for secure communication?
A [VPC endpoint](https://docs.aws.amazon.com/whitepapers/latest/aws-privatelink/what-are-vpc-endpoints.html)
enables users to privately connect to supported AWS services and VPC endpoint services powered by AWS PrivateLink.
In summary, PrivateLink can be set up manually on an individual basis, upon request. It also implies extra cost, because there is a cost associated with it in AWS, and it’s not included in the VictoriaMetrics Cloud offering.

In any case, it’s important to note that connecting via public access is always secured via TLS with all endpoints.

## How does VictoriaMetrics Cloud ensure data integrity and consistency?
We use the VictoriaMetrics Open Source project. To learn more, visit the [Open Source documentation](https://docs.victoriametrics.com/cluster-victoriametrics/#architecture-overview).

## How does VictoriaMetrics Cloud handle scalability within AWS resources?
VictoriaMetrics Cloud deployments run in isolated environments, so scaling can be done freely. We have processes that ensure zero-downtime in cluster setups and very low downtime for single setups.

## Are there latency or performance considerations when integrating?
* VictoriaMetrics Cloud deployments run in isolated environments, so there’s no interference between users and deployments are expected to run at high performance when compared, for example, with heavy-loaded on-prem setups.
* Users can choose between different AWS regions, and selecting one that is closer helps.
* The latency induced from running in the cloud is not noticeable for querying in dashboards or ingestion operation.

Should you need a different region, contact us at support-cloud@victoriametrics.com.

## What SLAs does VictoriaMetrics Cloud offer for availability and performance?
SLA are available on our web site: https://victoriametrics.com/legal/cloud/terms-of-service/#service-levels

## Does the VictoriaMetrics Cloud provide logging and monitoring capabilities?
Yes, logs and some of the metrics for your instances are available in the Victoria Metrics Console, we also provide alert notifications about issues with your instances.

## Can VictoriaMetrics Cloud integrate with AWS monitoring services like CloudWatch, X-Ray, or AWS Config?
We have integration with CloudWatch, you can find it in Console -> Integrations: https://console.victoriametrics.cloud/integrations/cloudwatch
Let us know if you need more integrations at support-cloud@victoriametrics.com.

## What troubleshooting mechanisms are in place for debugging issues?
In case of deployment issues, users are notified with alerts, which have recommendations for possible fixes. Instance logs are also available under the Logs tab (log messages also usually contains recommendations) and for instance metrics available in the Monitoring tab of each deployment.

Apart from that, there are other mechanisms:
* Cardinality explorer: https://docs.victoriametrics.com/#cardinality-explorer
* Query tracing: https://docs.victoriametrics.com/#query-tracing
* Top queries: https://docs.victoriametrics.com/#top-queries
* Active queries: https://docs.victoriametrics.com/#active-queries
* And other tools (https://docs.victoriametrics.com/#vmui) like Metric relabel debugger, Downsampling filters debugger, Retention filters debugger, Raw query view, etc…

Also, in case of problems, support is always available to help you at support-cloud@victoriametrics.com.

## What are the pricing models for VictoriaMetrics Cloud (subscription, usage-based, etc.)?
VictoriaMetrics Cloud pricing is based in tiers. Tiers are configured based on a handful of parameters. See [Tier Parameters](https://docs.victoriametrics.com/victoriametrics-cloud/tiers-parameters/) for more information.

Detailed and updated tier pricing can be checked in the console when [creating deployments](https://cloud.victoriametrics.com/deployments/create).

## Are there data transfer costs associated with VictoriaMetrics Cloud integrations?
Yes. We charge $0.09 per GB for external traffic, which matches AWS’ rate. Estimated traffic costs typically range from $1 to $30 per month, depending on deployment size and regular usage (such as data visualization, evaluation recording, and alerting rules and other integrations).

## Are there additional costs for API calls or storage?
VictoriaMetrics Cloud does not charge extra for API calls.
Regarding storage, the price is $1.46 for 10 Gb per Month. Since VictoriaMetrics Cloud is easy to
scale, we recommend users to expand storage resources with consumption, instead of allocating all storage space from the beginning.
We also offer deduplication and [cardinality explorer](https://docs.victoriametrics.com/#cardinality-explorer) mechanisms
to help reducing costs.

## Can VictoriaMetrics Cloud expenses be consolidated into my AWS bill?
Yes. You can subscribe via AWS marketplace (see payment methods [documentation](https://docs.victoriametrics.com/victoriametrics-cloud/billing/#aws-marketplace)).

## How does billing work?
See details in the [billing documentation and dedicated FAQ](https://docs.victoriametrics.com/victoriametrics-cloud/billing/).

## Where can I check the status of VictoriaMetrics Cloud?
We expose the status of the VictoriaMetrics Cloud service in https://status.victoriametrics.com/

## What's the Privacy Policy of VictoriaMetrics Cloud?
VictoriaMetrics Cloud Privacy Policy is available [here](https://cloud.victoriametrics.com/static/pdf/privacy_policy.pdf).

## Which are VictoriaMetrics Cloud Terms of Service?
VictoriaMetrics Cloud Terms of Service are publicly available [here](https://victoriametrics.com/legal/cloud/terms-of-service/), including [SLAs](https://victoriametrics.com/legal/cloud/terms-of-service/#service-levels).

