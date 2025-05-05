---
title : "Grafana"
menu:
  docs:
    parent: "integrations"
---

[Grafana](https://grafana.com/) is a popular open-source visualization and dashboarding tool. You can
use Grafana to query and visualize metrics stored in VictoriaMetrics Cloud using the built-in **Prometheus data source**.

This integration allows you to build powerful, customizable dashboards and monitor your systems in
real time using VictoriaMetrics as the backend.

## Integrating with Grafana

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **read access**, for querying metrics), follow the steps in the
[Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To connect Grafana with VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/grafana),
or follow this interactive guide:

<iframe 
    width="100%"
    style="aspect-ratio: 1/4;"
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/grafana" >
</iframe>
