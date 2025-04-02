---
weight: 3
title: Quick Start
menu:
  docs:
    parent: "get-started"
    weight: 3
aliases:
  - /victoriametrics-cloud/get-started/index.html
  - /victoriametrics-cloud/quickstart/index.html
  - /managed-victoriametrics/quickstart/index.html

---
Congratulations! You are just a few clicks away from running your favorite monitoring stack
without needing to worry about its maintenance, proper configuration, access protection,
software updates or backups. We take care of that so you can focus on what matters.

The process is very simple: once you are done with [registration](#registration), you'll be all set to
[create a deployment](#creating-deployments) and [start writing and reading data](#start-writing-and-reading-data)
right away.

Once the trial period ends, [adding a payment method](#adding-a-payment-method) will let you continue
using VictoriaMetrics Cloud.

## Registration

Start your registration process by visiting the [Sign Up](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_quickstart) page.
VictoriaMetrics Cloud supports registration via Sign up with Google Auth service or Email and password.

{{% collapse name="How to restore your password" %}}

> If you forgot your password, it can always be restored by clicking the `Forgot password?` link on the [Sign In](https://console.victoriametrics.cloud/signIn?utm_source=website&utm_campaign=docs_quickstart) page.
If you need assistance or have any questions, don't hesitate to contact our support team at support-cloud@victoriametrics.com.

{{% /collapse %}}

## Creating deployments

Creating VictoriaMetrics Cloud deployments is straightforward. Simply navigate
to the [Deployments](https://console.victoriametrics.cloud/deployments?utm_source=website&utm_campaign=docs_quickstart) page,
click on `Create`, pick a [tier](https://docs.victoriametrics.com/victoriametrics-cloud/tiers-parameters/),
and the instance will be up & running in a few seconds.

> To create your first deployment, click on `Start using VictoriaMetrics Cloud`.

### Customize your deployment

When creating a deployment, the following options are available:

| **Option**           | **Description**                    |
|------------------------------------|-----------------------------------|
| <nobr>**`Deployment name`**</nobr>    | A unique name for your deployment that will help you identify it.  |
| **`Single-node`**        | For affordable, performant deployments. |
| **`Cluster`**            | For highly available and multi-tenant deployments at scale.  |
| **`Region`**             | The cloud provider region where your deployment runs. For optimal performance and reduced traffic costs, select a region close to your application.  |
| **`Tier`**               | VictoriaMetrics Cloud offers a predefined set of instance sizes (or [tiers](https://docs.victoriametrics.com/victoriametrics-cloud/tiers-parameters/)) that cover most use cases. In this way, we can keep fixed pricing without surprises. Read [this guide](https://docs.victoriametrics.com/guides/understand-your-setup-size.html) to understand your setup size. Keep in mind that deployments may be [modified](#modifying-an-existing-deployment)!  |
| **`Retention`**          | The time, in months or days, you want to keep your metrics. Once set, VictoriaMetrics Cloud recommends storage size based on it. See this [note](#about-storage) for more information.  |
| **`Storage`**          |  Disk size for data storage. You always can expand disk size later. See this [note](#about-storage) for more information. |
| **`Deduplication`** | Deduplication handles redundant data in high-availability (HA) setups to retain only one sample per interval. For best results, set deduplication to match the collect metrics interval. If you have multiple intervals, set it to the shortest one. |
| <nobr>**`Maintenance Window`**</nobr> | We use this value as the preferred window for us to perform maintenance operations, such as upgrades, when needed. |

![Selecting a tier](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/create_deployment_form_down.webp "Selecting a tier")
<figcaption style="text-align: center; font-style: italic;">Selecting a tier</figcaption>

After selecting your desired configuration, you are set to `Create` your deployment. Once created, it will remain for a few seconds in `Provisioning` status while spinning-up. 
You'll also be notified via email once your deployment is ready to use.

{{% collapse name="Expand to learn more about retention and storage considerations" %}}

### About storage
* **Data point sizes** are approximated to 0.8 bytes, based on our own experience managing VictoriaMetrics Cloud. This magnitude is increases with **cardinality**. For high cardinality data, more storage is expected.
* **Long time retention**: for 6 months or more retention times, we recommend to start with a smaller storage size and increase it over time.
* **Storage size can be increased**, however, you cannot reduce it due to AWS limitations.
* **Enterprise features** like [downsampling](https://docs.victoriametrics.com/#downsampling) and [retention filters](https://docs.victoriametrics.com/#retention-filters) may dramatically help to optimize disk space.
* The **formula** we use for calculating the recommended storage can be found [here](https://docs.victoriametrics.com/guides/understand-your-setup-size/#retention-perioddisk-space).

> Feel free to adjust your deployment based on these recommendations.

{{% /collapse %}}

## Start writing and reading data

After the transition from `Provisioning` to `Running` state, the VictoriaMetrics Cloud deployment
is fully operational and ready to accept write and read requests. Writing and reading data in VictoriaMetrics Cloud is very simple.
Many integrations are supported. Comprehensive examples and guides may be found in the [integrations](https://cloud.victoriametrics.com/integrations?utm_source=website&utm_campaign=docs_quickstart) section.

> To read or write data into VictoriaMetrics Cloud, you just need to point your application to your deployment's `Access endpoint` and authorize with an `Access token`.

In brief, you will **only need to perform 2 steps**:
1. Obtain the **`Access endpoint`** for your deployment, which can be found in the [Deployments](https://console.victoriametrics.cloud/deployments?utm_source=website&utm_campaign=docs_quickstart) overview. Typically, it looks like: `https://<xxxx>.cloud.victoriametrics.com`.
2. Create or reuse an **`Access token`** to allow any application to read or write data into VictoriaMetrics Cloud. Just pick a `Name`, select read and/or write `Permission` and `Generate` it. For every deployment, you can `Generate tokens` in the `Access tokens` tab.

{{% collapse name="Expand to discover examples for vmagent, Prometheus, Grafana or any other software" %}}

### Examples for Reading and Writing data into VictoriaMetrics Cloud

Apart from the mentioned [integrations](https://cloud.victoriametrics.com/integrations?utm_source=website&utm_campaign=docs_quickstart) section,
you can always check for quick and easy Copy-paste examples by clicking on the three dots of the desired Access Token and select `Show examples`.

It will provide snippets like:

#### vmagent

```sh
./vmagent \
    --remoteWrite.url=https://<your_access_point>.cloud.victoriametrics.com/api/v1/write \
    --remoteWrite.bearerToken=********
```

#### Prometheus Configuration

```yaml
remote_write:
  - url: https://<your_access_point>.cloud-test.victoriametrics.com/api/v1/write
    authorization:
      credentials: ********
```

#### Grafana

* `Datasource url`: https://<your_access_point>.cloud.victoriametrics.com
* `Custom HTTP Header`: Authorization
* `Header value`: **********

![Deployment access write example](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/deployment_access_write_example.webp)
<figcaption style="text-align: center; font-style: italic;">Write configuration examples</figcaption>


{{% /collapse %}}


## Modifying an existing deployment

Remember that you can always add, remove or modify existing deployments by changing their configuration on the
deployment's page. It is important to know that downgrade for clusters is currently not available.

Additional configuration options may be found under `Advanced Settings`  where the following additional parameters can be set:

* [`Deduplication`](https://docs.victoriametrics.com/cluster-victoriametrics/#deduplication) defines interval when deployment leaves a single raw sample with the biggest timestamp per each discrete interval;
* `Maintenance Window` when deployment should start an upgrade process if needed;
* `Settings` to define VictoriaMetrics deployment flags, depending on your deployment type: [Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#list-of-command-line-flags) or [Single-node](https://docs.victoriametrics.com/single-server-victoriametrics/#list-of-command-line-flags).

> These updates require a deployment restart and may result in a short downtime for **single-node** deployments.


## Adding a payment method

VictoriaMetrics Cloud supports different payment options. You can found more information under the [Billing](/victoriametrics-cloud/billing/) section.

To add your payment method, navigate to the VictoriaMetrics Cloud [Billing](https://console.victoriametrics.cloud/billing?utm_source=website&utm_campaign=docs_quickstart)
page, and go to the `Payment methods` tab. There, you'll be able to add a payment method by:

1. **Bank card**: fill required fields
2. **AWS Marketplace**: link your AWS billing account via AWS Marketplace. This option will redirect you to the [AWS VictoriaMetrics Cloud product page](https://aws.amazon.com/marketplace/pp/prodview-atfvt3b73m2z4), where you can easily `Subscribe` to VictoriaMetrics Cloud. You'll be redirected back to VictoriaMetrics Cloud [Billing page](https://console.victoriametrics.cloud/billing?utm_source=website&utm_campaign=docs_quickstart) by clicking on `Set up your account`.

If you add both payment methods, you can easily switch between them by selecting your preferred option.

> [!NOTE] What happens if a payment method is not configured?
> After the trial period expires, deployments will be stopped and deleted if no payment methods are found for your account.
> If you need assistance or have any questions, don't hesitate to contact our support team at support-cloud@victoriametrics.com.
