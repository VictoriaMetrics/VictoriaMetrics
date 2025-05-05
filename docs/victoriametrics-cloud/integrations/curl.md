---
title : "CURL"
menu:
  docs:
    parent: "integrations"
---

You can use [curl](https://curl.se/) to interact with VictoriaMetrics Cloud for both **pushing** metrics and **querying**
stored data using HTTP API endpoints. This makes it a simple and flexible option for testing or basic
integrations.

## Integrating with CURL

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication.
The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access** for pushing data or **read access** for querying),
follow the steps in the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To integrate CURL with VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/curl),
or simply follow this interactive guide:


<iframe 
    width="100%"
    style="aspect-ratio: 1/2;"
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/curl" >
</iframe>
