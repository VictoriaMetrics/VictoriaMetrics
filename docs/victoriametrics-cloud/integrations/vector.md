---
title : "Vector"
menu:
  docs:
    identifier: victoriametrics-cloud-integrations-vector
    parent: "integrations"
---

[Vector](https://vector.dev/) is an observability data pipeline that can collect,
transform, and route metrics and logs. For metrics, Vector can be configured to send data to VictoriaMetrics
Cloud using the **Prometheus remote write** protocol.

This integration is useful to route metrics to VictoriaMetrics Cloud for storage and analysis.

## Integrating with Vector

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access**, since Vector pushes metrics), follow the steps in
the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To set up Vector to forward metrics to VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/vector),
or follow this interactive guide:

<iframe 
    width="100%"
    height="1250" 
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/vector" 
    style="background: white;" >
</iframe>
