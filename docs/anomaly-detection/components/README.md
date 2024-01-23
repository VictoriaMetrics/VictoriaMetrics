---
title: Components
weight: 2
menu:
  docs:
    identifier: "vmanomaly-components"
    parent: "anomaly-detection"
    weight: 2
aliases:
  - /anomaly-detection/components/
  - /anomaly-detection/components/index.html
---

# Components

This chapter describes different components, that correspond to respective sections of a config to launch VictoriaMetrics Anomaly Detection (or simply [`vmanomaly`](/anomaly-detection/overview.html)) service:

- [Model(s) section](models.html) - Required
- [Reader section](reader.html) - Required
- [Scheduler section](scheduler.html) - Required
- [Writer section](writer.html) - Required
- [Monitoring section](monitoring.html) -  Optional

> **Note**: starting from [v1.7.0](/anomaly-detection/CHANGELOG.html#v172), once the service starts, automated config validation is performed. Please see container logs for errors that need to be fixed to create fully valid config, visiting sections above for examples and documentation.

Below, you will find an example illustrating how the components of `vmanomaly` interact with each other and with a single-node VictoriaMetrics setup.

> **Note**: [Reader](/anomaly-detection/components/reader.html#vm-reader) and [Writer](/anomaly-detection/components/writer.html#vm-writer) also support [multitenancy](/Cluster-VictoriaMetrics.html#multitenancy), so you can read/write from/to different locations - see `tenant_id` param description.

<img alt="vmanomaly-components" src="vmanomaly-components.webp" width="800px"/>