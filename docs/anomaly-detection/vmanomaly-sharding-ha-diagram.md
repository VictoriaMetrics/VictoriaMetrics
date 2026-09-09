---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

The global configuration is split into `N` independently valid sub-configurations. Deterministic placement distributes them across `K` members, and each sub-configuration is assigned to exactly `R` members when replication is enabled. Member `k` processes only its assigned subset.

![vmanomaly sharding and high availability: a global configuration is split into N sub-configurations and replicated across K members](/anomaly-detection/vmanomaly-sharding-ha-diagram.svg)
{style="display:block; width:50%; min-width:320px; margin:1.5rem auto"}
