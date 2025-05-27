---
title : "Kubernetes"
menu:
  docs:
    parent: "integrations"
---

VictoriaMetrics Cloud supports monitoring Kubernetes clusters using the
[VictoriaMetrics Kubernetes Stack](https://docs.victoriametrics.com/helm/victoriametrics-k8s-stack/), a Helm-based
deployment that includes preconfigured components for efficient and scalable metrics collection in Kubernetes environments.

This stack collects metrics from the cluster, nodes, and workloads, and forwards them to VictoriaMetrics Cloud using
`vmagent`, along with built-in dashboards and alerting capabilities.

## Integrating with Kubernetes

All VictoriaMetrics Cloud integrations, including this one, require an access token for authentication. The configuration examples below contain two placeholders: `<DEPLOYMENT_ENDPOINT_URL>` and `<YOUR_ACCESS_TOKEN>`. These need to be replaced with your actual access token.

To generate your access token (with **write access**, as metrics will be pushed), follow the steps in the [Access Tokens documentation](https://docs.victoriametrics.com/victoriametrics-cloud/deployments/access-tokens).

To set up Kubernetes monitoring using the VictoriaMetrics stack, visit the [cloud console](https://console.victoriametrics.cloud/integrations/kubernetes), or follow this interactive guide:


<iframe 
    width="100%" 
    height="2100" 
    name="iframe" 
    id="integration" 
    frameborder="0"
    src="https://console.victoriametrics.cloud/public/integrations/kubernetes" 
    style="background: white;" >
</iframe>
