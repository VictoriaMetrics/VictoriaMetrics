---
weight: 5
title: Organizations in VictoriaMetrics Cloud
menu:
  docs:
    parent: account-management
    weight: 5
    name: Organizations
---

Organizations in VictoriaMetrics Cloud are designed to streamline team collaboration, improve
access control, and simplify scaling observability across multiple teams and environments.

By using Organizations, users can invite collaborators, assign specific roles and permissions,
and organize deployments under a structured access model. This provides a more secure and
efficient way to manage access, scale operations, and maintain governance.

## Getting started with Organizations
New users of VictoriaMetrics Cloud registered via the [SignUp](https://console.victoriametrics.cloud/signUp)
page are automatically enrolled into a new Organization, where they can invite other new or
existing users.


## Working with Organizations

The left navigation menu of the VictoriaMetrics Cloud console is divided into Services (top) and Organizations (bottom).
All features described here are easily accessible through that menu.

{{% collapse name="User Management" %}}

The `User Management` page inside the Organizations menu allows to:
- Invite new users to collaborate
- Check activity, creation time and authentication methods used by users part of the Organization
- Manage other users

Organization `Admins` can perform the following `Actions` on other existing users:
  - Manage their [`roles`](https://docs.victoriametrics.com/victoriametrics-cloud/account-management/roles-and-permissions/)
  - `Deactivate` or `Activate` them: Deactivated users are still part of VictoriaMetrics Cloud but cannot perform actions inside the Organization
  - `Delete` them from the Organization

{{% /collapse %}}


{{% collapse name="API Keys" %}}

[API Keys](https://docs.victoriametrics.com/victoriametrics-cloud/api/) are needed to enforce
authentication in programmatic actions (for example, in scripts) to interact with VictoriaMetrics Cloud.
The API itself is documented in the [api-docs](https://console.victoriametrics.cloud/api-docs) page.

In the [API Keys](https://console.victoriametrics.cloud/api_keys) page, Organization Admins can:
* Create API Keys, giving them an easily identifiable `Name`, set the `Lifetime`, `Permissions` (Read, Write or both), and grant permissions to all or specific VictoriaMetrics Cloud deployments.
* Check existing API Keys relevant information
* Revoke previously generated API Keys

{{% /collapse %}}

{{% collapse name="Billing" %}}

Centralized billing information can be accessed through the [Billing Page](https://console.victoriametrics.cloud/billing).
Here Organization `Admins` can check usage, manage Payment Methods, download invoices and check ongoing spends.

For more billing related information, read the [Billing documentation](https://docs.victoriametrics.com/victoriametrics-cloud/billing/) page.

{{% /collapse %}}

{{% collapse name="Audit Logs" %}}

VictoriaMetrics Cloud provides centralized access to [Audit Logs](https://console.victoriametrics.cloud/audit) for Organizations.
Here, `Admins` can check events performed by other Organization users within VictoriaMetrics Cloud.
Audit logs can be filtered by Action, Email or Date.

VictoriaMetrics Cloud also enables Exporting Audit Logs as CSV.

{{% /collapse %}}

{{% collapse name="Details" %}}

Organization `Admins` can change their Organization name or leave an Organization in the `Details` [page](https://console.victoriametrics.cloud/organization).

{{% /collapse %}}

