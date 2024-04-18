---
sort: 12
weight: 12
title: VMServiceScrape
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 12
aliases:
  - /operator/resources/vmservicescrape.html
---

# VMServiceScrape

The `VMServiceScrape` CRD allows to define a dynamic set of services for monitoring. Services
and scraping configurations can be matched via label selections. This allows an organization to introduce conventions
for how metrics should be exposed. Following these conventions new services will be discovered automatically without
need to reconfigure.

`VMServiceScrape` object generates part of [VMAgent](./vmagent.md) configuration with 
[kubernetes service discovery](https://docs.victoriametrics.com/sd_configs.html#kubernetes_sd_configs) targets by corresponding `Service`.
It has various options for scraping configuration of target (with basic auth,tls access, by specific port name etc.).

Monitoring configuration based on  `discoveryRole` setting. By default, `endpoints` is used to get objects from kubernetes api.
It's also possible to use `discoveryRole: service` or `discoveryRole: endpointslices`.

`Endpoints` objects are essentially lists of IP addresses.
Typically, `Endpoints` objects are populated by `Service` object. `Service` object discovers `Pod`s by a label
selector and adds those to the `Endpoints` object.

A `Service` may expose one or more service ports backed by a list of one or multiple endpoints pointing to
specific `Pod`s. The same reflected in the respective `Endpoints` object as well.

The `VMServiceScrape` object discovers `Endpoints` objects and configures [VMAgent](./vmagent.md) to monitor `Pod`s.

The `Endpoints` section of the `VMServiceScrapeSpec` is used to configure which `Endpoints` ports should be scraped.
For advanced use cases, one may want to monitor ports of backing `Pod`s, which are not a part of the service endpoints.
Therefore, when specifying an endpoint in the `endpoints` section, they are strictly used.

**Note:** `endpoints` (lowercase) is the field in the `VMServiceScrape` CRD, while `Endpoints` (capitalized) is the Kubernetes object kind.

Both `VMServiceScrape` and discovered targets may belong to any namespace. It is important for cross-namespace monitoring
use cases, e.g. for meta-monitoring. Using the `serviceScrapeSelector` of the `VMAgentSpec`
one can restrict the namespaces from which `VMServiceScrape`s are selected from by the respective [VMAgent](./vmagent.md) server.
Using the `namespaceSelector` of the `VMServiceScrape` one can restrict the namespaces from which `Endpoints` can be
discovered from. To discover targets in all namespaces the `namespaceSelector` has to be empty:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  name: example-service-scrape
spec:
  namespaceSelector: {}
  # ...
```

More information about selectors you can find in [this doc](./vmagent.md#scraping).

## Specification

You can see the full actual specification of the `VMServiceScrape` resource in
the **[API docs -> VMServiceScrape](../api.md#vmservicescrape)**.

Also, you can check out the [examples](#examples) section.

## Migration from Prometheus

The `VMServiceScrape` CRD from VictoriaMetrics Operator is a drop-in replacement 
for the Prometheus `ServiceMonitor` from prometheus-operator.

More details about migration from prometheus-operator you can read in [this doc](../migration.md).

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMServiceScrape
metadata:
  name: example-app
  labels:
    team: frontend
spec:
  selector:
    matchLabels:
      app: example-app
  endpoints:
  - port: web
```
