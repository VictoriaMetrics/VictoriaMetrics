---
sort: 1
title: Docs
weight: 5
menu:
  docs:
    identifier: vmanomaly-docs
    parent: "anomaly-detection"
    weight: 5
aliases:
  - /anomaly-detection/docs/
  - /anomaly-detection/docs/index.html
---

# Docs

This chapter describes different sections of a config to launch VictoriaMetrics Anomaly Detection (or simply [`vmanomaly`](/vmanomaly.html)) service:

- [Model(s) section](models/README.md) - Required
- [Reader section](reader.html) - Required
- [Scheduler section](scheduler.html) - Required
- [Writer section](writer.html) - Required
- [Monitoring section](monitoring.html) -  Optional


> **Note**: starting from [v1.7.0](../CHANGELOG.md#v172), once the service starts, automated config validation is performed. Please see container logs for errors that need to be fixed to create fully valid config, visiting sections above for examples and documentation.