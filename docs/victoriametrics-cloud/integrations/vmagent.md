---
title : "VMAgent"
menu:
  docs:
    parent: "integrations"
---

[VMAgent](https://docs.victoriametrics.com/victoriametrics/vmagent/) is a lightweight agent
designed to collect metrics from various sources, apply [relabeling and filtering](https://docs.victoriametrics.com/victoriametrics/relabeling/) rules, and
forward the data to storage systems. It supports both the Prometheus `remote_write` protocol and
the [VictoriaMetrics `remote_write` protocol](https://docs.victoriametrics.com/victoriametrics/vmagent/#victoriametrics-remote-write-protocol)
for sending data.

This makes VMAgent ideal for centralized metric collection and forwarding in a resource-efficient way.

## Integrating VMAgent

All VictoriaMetrics Cloud integrations, including this one, require an access token for
authentication. The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and
`<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (for write access in this case), follow the steps in the
[Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To integrate VMAgent with VictoriaMetrics Cloud, visit the [cloud console](https://console.victoriametrics.cloud/integrations/vmagent),
or simply follow this interactive guide:

<iframe 
    width="100%"
    style="aspect-ratio: 1/3;"
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/vmagent" >
</iframe>
