---
title : "Perses"
menu:
  docs:
    parent: "integrations"
---
[Perses](https://perses.dev/) is an open-source visualization and dashboarding tool, designed for
simplicity, performance, and scalability.

VictoriaMetrics Cloud can be used as a data source in Perses via the **Prometheus-compatible query API**,
allowing you to create dashboards and monitor time series data with a modern and lightweight interface.

## Integrating with Perses

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and `<YOUR_ACCESS_TOKEN>`.
These need to be replaced with your actual access token.

To generate your access token (with **read access**, for querying metrics), follow the steps in the
[Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To connect Perses with VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/perses),
or follow this interactive guide:

<iframe 
    width="100%"
    style="aspect-ratio: 1/4;"
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/perses" >
</iframe>
