---
title : "Telegraf"
menu:
  docs:
    parent: "integrations"
---

[Telegraf](https://www.influxdata.com/time-series-platform/telegraf/) is a plugin-driven server agent
used for collecting and reporting metrics. It supports a wide range of input plugins and can be
configured to send metrics to VictoriaMetrics Cloud using the **Prometheus remote write** protocol.

This integration is ideal for environments where Telegraf is already used to gather system, application,
or custom metrics.

## Integrating with Telegraf

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access**, as Telegraf pushes metrics), follow the steps in
the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To configure Telegraf with VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/telegraf),
or follow this interactive guide:


<iframe 
    width="100%"
    height="850" 
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/telegraf" 
    style="background: white;" >
</iframe>
