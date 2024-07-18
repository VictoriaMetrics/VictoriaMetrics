---
# sort: 14
title: Data Ingestion 
weight: 0
menu:
  docs:
    parent: 'victoriametrics'
    identifier: 'data-ingestion'
    weight: 7
aliases:
- /data-ingestion.html
- /data-ingestion.html
- /dataingestion/
---

# Data Ingestion
In This Folder you will find instructions for sending data to VictoriaMetrics from a variety of platforms.
If your tool is not listed it is likely you can ingest your data into VictoriaMetrics using one of the protocols listed in our [Prominent features]({{< ref "/Single-server-VictoriaMetrics.md#prominent-features" >}}) section.

If you are unsure what port number to use when pushing data to VictoriaMetrics single node, vminsert, vmagent, and vmauth we have listed the default ports below.

- VictoriaMetrics Single: 8428
- vmagent: 8429
- vmauth: 8427
- vminsert: 8482

In the rest of the documentation we will assume you have configured your push endpoint to use TLS/SSL on port 443 so the urls in the rest of the documentation will look like `https://<victoriametrics_url>` instead of `http://<victoriametrics_url>:8428` for VictoriaMetrics single.

## Documented Collectors/Agents
* [Telegraf]({{< relref "Telegraf.md" >}})
* [Vector]({{< relref "Vector.md" >}})

## Supported Platforms
* [Proxmox Virtual Environment and Proxmox Backup Server]({{< relref "Proxmox.md" >}})

