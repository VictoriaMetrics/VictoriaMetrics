---
sort: 14
weight: 14
title: VMStaticScrape
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 14
aliases:
  - /operator/resources/vmstaticscrape.html
---

# VMStaticScrape

The `VMStaticScrape` CRD provides mechanism for scraping metrics from static targets, configured by CRD targets.

`VMStaticScrape` object generates part of [VMAgent](./vmagent.md) 
configuration with [static "service discovery"](https://docs.victoriametrics.com/sd_configs.html#static_configs).
It has various options for scraping configuration of target (with basic auth,tls access, by specific port name etc.).

By specifying configuration at CRD, operator generates config 
for [VMAgent](./vmagent.md) and syncs it. 
It's useful for external targets management, when service-discovery is not available. 
`VMAgent` `staticScrapeSelector` must match `VMStaticScrape` labels.

More information about selectors you can find in [this doc](./vmagent.md#scraping).

## Specification

You can see the full actual specification of the `VMStaticScrape` resource in
the **[API docs -> VMStaticScrape](../api.md#vmstaticscrape)**.

Also, you can check out the [examples](#examples) section.

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMStaticScrape
metadata:
  name: vmstaticscrape-sample
spec:
  jobName: static
  targetEndpoints:
    - targets: ["192.168.0.1:9100", "196.168.0.50:9100"]
      labels:
        env: dev
        project: operator
```
