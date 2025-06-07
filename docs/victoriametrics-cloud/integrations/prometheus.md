---
title : "Prometheus (remote write)"
menu:
  docs:
    parent: "integrations"
---

VictoriaMetrics Cloud supports integration with [Prometheus](https://prometheus.io/) using the
**remote write** protocol, allowing you to forward metrics collected by Prometheus to VictoriaMetrics
Cloud for long-term storage and advanced querying.

This setup enables you to keep using your existing Prometheus instances for local scraping and rule
evaluation, while offloading storage and visualization to VictoriaMetrics Cloud.

## Integrating with Prometheus Remote Write

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and `<YOUR_ACCESS_TOKEN>`.
These need to be replaced with your actual access token.

To generate your access token (with **write access**, since Prometheus pushes metrics), follow the steps
in the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To configure Prometheus to remote write to VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/prometheus),
or follow this interactive guide:

<iframe 
    width="100%"
    height="1200" 
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/prometheus" 
    style="background: white;" >
</iframe>
