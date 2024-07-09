---
title: Proxmox
weight: 1
sort: 1
menu:
  docs:
    identifier: "proxmox"
    parent: "data-ingestion"
    weight: 1
    # sort: 1
aliases:
  - /data-ingestion/proxmox.html
  - /data-ingestion/Proxmox.html
---

# Proxmox Data Ingestion
Since Proxmox Virtual Environment(PVE) and Proxmox Backup Server(PBS) support sending data using the InfluxDB We can use the InfluxDB write support built into VictoriaMetrics
Currently PVE and PBS only support using an Authorization Token for authentication and does not support basic auth or a username and password.

## Proxmox Virtual Environment (PVE)
If want help Sending your data to Managed VictoriaMetrics check out [our blog](https://victoriametrics.com/blog/proxmox-monitoring-with-dbaas/).

1. Login to PVE as an administrator
2. Go to DataCenter > MetricServer > Add > InfluxDB
3. Set the parameters as follows
  * Name: VictoriaMetrics (can be changed to any string)
  * Server: the hostname or IP of your VictoriaMetrics Instance
  * Port: This will vary depending on where you were you sending the data and your settings but here are the defaults for the various components
    * 8428 for sending directly to VMInsert or VMSingle
    * 8427 for VMAuth
    * 8429 for VMAgent
  * Protocol: use HTTPS if you have TLS/SSL configured otherwise use HTTPS
  * Organization: leave empty since it doesn't get used
  * Bucket: leave empty since it doesn't get used
  * Token: Your Token From VMAuth or leave blank if you don't have authentication enabled
  * If you need to ignore TLS/SSL errors check the advanced box and uncheck the verify certificate box
4. Run `` in VMUI or in the explore view in Grafana to verify are being received by VictoriaMetrics


## Proxmox Backup Server (PBS)
1. Login to PBS as an administrator
2. Go to Configuration > Metrics Server > Add > InfluxDB
3.  Set the parameters as follows
  * Name: Victoriametrics (can be set to any string)
  * URL: http(s)://<ip_or_host>:<port>
    * set the URL to https if you have TLS enabled and http if you do not
    * Port: This will vary depending on where you were you sending the data and your settings but here are the defaults for the various components
      * 8428 for sending directly to VMInsert or VMSingle
      * 8427 for VMAuth
      * 8429 for VMAgent
  * Organization: leave empty since it doesn't get used
  * Bucket: leave empty since it doesn't get used
  * Token: Your Token From VMAuth or leave blank if you don't have authentication enabled
4. Run `` in VMUI or in the explore view in Grafana to verify


# References
- [Blog Post for configuring Managed VictoriaMetrics and Proxmox VE](https://victoriametrics.com/blog/proxmox-monitoring-with-dbaas/)
