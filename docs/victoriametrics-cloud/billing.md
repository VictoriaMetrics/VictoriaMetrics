---
weight: 10
title: VictoriaMetrics Cloud Billing
menu:
  docs:
    parent: "cloud"
    weight: 10
    name: Billing
---

VictoriaMetrics Cloud charges for three key components:

- **Compute**: The cost of deployment installation.
- **Storage**: The storage used by the deployment.
- **Network**: External (egress) network usage.

This breakdown will help you to better understand and manage your costs. Usage data is sent hourly to the payment provider (AWS or Stripe). Detailed billing information is available via the [Billing Page](https://console.victoriametrics.cloud/billing) of your VictoriaMetrics Cloud account.

Each deployment operates with predefined configurations and limits, protecting you from unexpected overages caused by factors such as:

* Data ingestion spikes.
* Cardinality explosions.
* Accidental heavy queries.

This ensures predictable costs and proactive alerts for workload anomalies.

__Note__: VictoriaMetrics Cloud does not store or process your payment information. We rely on trusted API providers (Stripe, AWS) for secure payment processing.

## Pricing

Pricing begins at **$190/month** for the Starter Tier. To view other tiers and their costs, navigate to the [Create New Deployment](https://console.victoriametrics.cloud/deployments/create) section in the VictoriaMetrics Cloud application.

Our aim is to make pricing information easy to access and understand. If you have any questions or feedback on our pricing, please contact us.


## Usage Reports

The [Usage Reports](https://console.victoriametrics.cloud/billing/usage) section in the billing area provides a breakdown of:

* Storage Costs
* Compute Costs
* Networking Costs
* Applied Credits

Your Final Monthly Cost is calculated as `usage - credits` and reflects the amount billed by your payment provider.

A graph is also available to display the daily cost breakdown for the selected month.


## Payment Methods

VictoriaMetrics Cloud supports the following payment options:

- Credit Card
- AWS Marketplace
- ACH Transfers

You can add multiple payment methods and set one as the primary. Backup payment methods are used if the primary fails. More details are available via the [Payment Methods](https://console.victoriametrics.cloud/billing) tab of the Billing Page.

### Credit Card

Credit cards can be added through [Stripe](https://stripe.com/) integration.

### AWS Marketplace

Payments made via [AWS Marketplace](https://aws.amazon.com/marketplace/pp/prodview-atfvt3b73m2z4?sr=0-1&ref_=beagle&applicationId=AWSMPContessa) include billing details in the AWS portal. AWS finalizes monthly bills at the start of the next month, typically charging between the 3rd and 5th business day. Visit the [AWS Knowledge Center](https://aws.amazon.com/premiumsupport/knowledge-center/) for more information.

### ACH Transfers

ACH payments are supported. Contact [VictoriaMetrics Cloud Support](https://docs.victoriametrics.com/victoriametrics-cloud/support/) for setup assistance.



## Invoices

[Invoices](https://console.victoriametrics.cloud/billing/invoices) are emailed monthly to users who pay via Credit Card or ACH Transfers. Notification email addresses can be updated in the [VictoriaMetrics Cloud Notifications](https://docs.victoriametrics.com/victoriametrics-cloud/setup-notifications/) section.

Invoices are also accessible on the Invoices Page, which provides:

* Invoice Period
* Invoice Status
* Downloadable PDF Links

For AWS Marketplace billing, check the AWS Portal for invoice information.

---

## FAQ

### What billing options does VictoriaMetrics Cloud support?

* Monthly Billing: Pay-as-you-go.
* Annual/Multi-Year Contracts: Available via AWS or ACH transfers.

For more information, contact sales@victoriametrics.com.

### How is deployment usage metered?

Usage is metered hourly.

### Do you charge for backups?

No, backups are provided at no additional cost.

### How long is the billing cycle?

Although usage is metered hourly, billing is conducted monthly. The billing date corresponds to the registration date. For example, if you registered on December 5, you will be billed on the 5th of each subsequent month.

### Can you help reduce my costs?

We recommend using Enterprise features such as [downsampling](https://docs.victoriametrics.com/#downsampling) and [retention filters](https://docs.victoriametrics.com/#retention-filters) for cost optimization. Contact [VictoriaMetrics Cloud Support](https://docs.victoriametrics.com/victoriametrics-cloud/support/) for assistance.

### I want to extend my trial or get more credits. What should I do?

Contact [VictoriaMetrics Cloud Support](https://docs.victoriametrics.com/victoriametrics-cloud/support/) , and we’ll help extend your trial or provide additional credits.

### How do you charge for spikes in load?

We don’t charge for spikes. Each deployment has predefined configurations and limits. If a deployment cannot handle a spike, you will receive an alert, allowing you to take proactive measures.

