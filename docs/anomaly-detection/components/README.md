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


> **Note**: starting from [v1.7.0](../CHANGELOG.md#v172), once the service starts, automated config validation is performed. Please see container logs for errors that need to be fixed to create fully valid config, visiting sections above for examples and documentation.