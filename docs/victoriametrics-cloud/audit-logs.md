---
weight: 9
title: VictoriaMetrics Cloud Audit Logs
menu:
  docs:
    parent: "cloud"
    weight: 8
    name: Audit Logs
---
An [**audit log**](https://console.victoriametrics.cloud/audit) is a record of user and system activities within an organization. It captures details of who performed an action, what was done, and when it occurred. Audit logs are essential for security, compliance, and troubleshooting processes.

## Cloud Audit Log Scopes

VictoriaMetrics Cloud provides two scopes for audit logs:

1. **Organization-Level Audit Logs**  
   These logs record all activities at the organization level, such as user logins, token reveals, updates to payment information, and deployments being created or destroyed.
2. **Deployment-Level Audit Logs**  
   These logs record activities related to a specific deployment only, such as changes to deployment parameters, creating or deleting access tokens, and modifying alerting or recording rules.

## Example Log Entry

* **Time**: 2024-10-0515:40 UTC
* **Email**: cloud-admin@victoriametrics.com
* **Action**: cluster updated: production-platform, changed properties: vmstorage settings changed: disk size changed from 50.0TB to 80.0TB,

## Filtering

The audit log page offers filtering options, allowing you to filter logs by time range, actor, or perform a full-text search by action.

## Export to CSV

The Export to CSV button on the audit log page allows you to export the entire audit log as a CSV file.

Filtering does not affect the export; you will always receive the entire audit log in the exported file.

