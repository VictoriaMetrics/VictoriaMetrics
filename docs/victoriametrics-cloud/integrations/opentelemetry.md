---
title : "OpenTelemetry"
menu:
  docs:
    parent: "integrations"
---

VictoriaMetrics Cloud supports integration with the [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
for ingesting metrics using the OpenTelemetry Protocol (OTLP).

You can deploy the OpenTelemetry Collector using either the **Helm chart** or the **Operator**,
depending on your preference and environment, to collect, process, and
forward observability data from a wide variety of sources into VictoriaMetrics Cloud.

## Integrating with OpenTelemetry

All VictoriaMetrics Cloud integrations, including this one, require an access token for
authentication. The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>`
and `<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access**, as metrics are pushed to VictoriaMetrics Cloud),
follow the steps in the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To integrate OpenTelemetry Collector with VictoriaMetrics Cloud, visit the
[cloud console](https://console.victoriametrics.cloud/integrations/opentelemetry), or follow this interactive guide:

<iframe 
    width="100%"
    height="1800" 
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/opentelemetry" 
    style="background: white;" >
</iframe>
