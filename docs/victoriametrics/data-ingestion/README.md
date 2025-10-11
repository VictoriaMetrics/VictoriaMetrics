---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
In This Folder you will find instructions for sending data to VictoriaMetrics from a variety of platforms.
If your tool is not listed it is likely you can ingest your data into VictoriaMetrics using one of the protocols listed
in our [Prominent features](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prominent-features) section.

If you are unsure what port number to use when pushing data to VictoriaMetrics single node, vminsert, vmagent, and vmauth we have listed the default ports below:
- VictoriaMetrics Single: 8428
- vmagent: 8429
- vmauth: 8427
- vminsert: 8480

In the rest of the documentation we will assume you have configured your push endpoint to use TLS/SSL on port 443 
so the urls in the rest of the documentation will look like `https://<victoriametrics_url>` instead of `http://<victoriametrics_url>:8428` for VictoriaMetrics single.

## Documented Collectors/Agents

- [Telegraf](https://docs.victoriametrics.com/victoriametrics/data-ingestion/telegraf/)
- [Vector](https://docs.victoriametrics.com/victoriametrics/data-ingestion/vector/)
- [vmagent](https://docs.victoriametrics.com/victoriametrics/data-ingestion/vmagent/)
- [Grafana Alloy](https://docs.victoriametrics.com/victoriametrics/data-ingestion/alloy/)
- [Prometheus](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/)


## Supported Platforms

- [Proxmox Virtual Environment and Proxmox Backup Server](https://docs.victoriametrics.com/victoriametrics/data-ingestion/proxmox/)
