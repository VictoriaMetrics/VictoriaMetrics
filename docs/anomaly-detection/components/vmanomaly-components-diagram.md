---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

The required path is `config.yml` → Scheduler → Reader → Model → Writer. The Reader queries the configured VictoriaMetrics, VictoriaLogs, or VictoriaTraces datasource; the Writer stores inferred anomaly scores in VictoriaMetrics. Monitoring is optional and can push metrics or expose them for collection.

Solid nodes and arrows show the required anomaly-detection path. Dashed nodes and arrows show optional self-monitoring integrations.

![vmanomaly component interaction: the Scheduler starts Reader, Model, and Writer work against a configured datasource; Monitoring is optional](/anomaly-detection/components/vmanomaly-components.svg)
{style="display:block; width:80%; min-width:320px; margin:1.5rem auto"}
