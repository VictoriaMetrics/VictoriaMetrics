---
weight: 5
title: Migration
menu:
  docs:
    identifier: "vmanomaly-migration"
    parent: "anomaly-detection"
    weight: 5
tags:
  - metrics
  - enterprise
  - migration
aliases:
- /anomaly-detection/migration/
- /anomaly-detection/migration/index.html
---

## Introduction

This document provides guidelines for migrating to the latest version of [VictoriaMetrics Anomaly Detection](https://docs.victoriametrics.com/anomaly-detection/) (`vmanomaly`). It covers the key changes, compatibility considerations, and best practices to ensure a smooth transition for [stateful](#stateful-mode) and [stateless](#stateless-mode) modes of operation.

> **Upgrading to [v1.27.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270) or newer is recommended to benefit from simplified migration process.**

## Dry Run

The `--dryRun` [command-line argument](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments) allows {{% available_from "v1.27.0" anomaly %}} to simulate the migration process without making any actual changes. This is useful for identifying potential issues and understanding the impact of the migration before applying it, e.g. dropping of existing state database or on-disk artifacts for all (or some) of the configured models and data. Starting from version [v1.27.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270), the upgrade impact to any new version can be assessed by running `vmanomaly` with the `--dryRun` flag **automatically**. Downgrade check from v1.27.0 (or newer) to earlier versions than [v1.25.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1253) **requires setting env variable** `VMANOMALY_STATE_VERSION_OVERRIDE=<version>`, e.g.:

```bash
export VMANOMALY_STATE_VERSION_OVERRIDE=1.25.2
```

## Compatibility Matrix

This section outlines the compatibility of different `vmanomaly` versions with various components, including data, models, and configuration formats, for both [stateful](#stateful-mode) and [stateless](#stateless-mode) modes.

> Refer to the **global** [changelog](https://docs.victoriametrics.com/anomaly-detection/changelog/) for detailed information on changes in each version. Use `--dryRun` {{% available_from "v1.27.0" anomaly %}} mode to check for compatibility issues before performing the actual migration. See [Dry Run](#dry-run) section for more details.

### Stateful Mode

> Used if `settings.restore_state` is set to `true`. See argument details in the [configuration documentation](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration).

There are 2 types of compatibilitity to consider when migrating in stateful mode:
- **Global (in)compatibility**: The new version can seamlessly read and utilize the existing state without any modifications or data loss. Or, in case of incompatibility, the existing state must be dropped completely to proceed with the migration.
- **Component (in)compatibility**: The new version may introduce changes that affect specific components (e.g., specific models, data formats) but can still operate with the existing state with some adjustments or drop of incompatible on disk artifacts.

| Group start | Group end | Compatibility | Notes |
|---------|--------- |------------|-------|
| [v1.28.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1280) | Latest* | Fully Compatible | Just a placeholder for new releases |
| [v1.26.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1262) | [v1.28.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1280) | Fully Compatible | [v1.28.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1280) introduced [rolling](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models) model class drop in favor of [online](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) models (`rolling_quantile` and `std` models), however, it does not impact compatibility, as artifacts were not produced by default for rolling models. |
| [v1.25.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1253) | [v1.26.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270) | Partially Compatible* | [v1.25.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1253) introduced `forecast_at` argument for base [univariate](https://docs.victoriametrics.com/anomaly-detection/components/models/#univariate-models) and `Prophet` [models](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet), however, itself remains backward-reversible from newer states like [v1.26.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1262), [v1.27.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270). (All models except `isolation_forest_multivariate` class will be dropped) |
| [v1.25.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1251) | [v1.25.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1252) | Fully Compatible | In [v1.25.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1251) there was a change to `vmanomaly.db` metadata database format, so migrating from v1.24.0-v1.25.0 requires deletion of a state, see note above the table |
| [v1.24.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1241) | [v1.25.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1250) | Partially Compatible* | In [v1.25.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1250) there were changes to **data dump layout** and to `online_quantile` and `isolation_forest_multivariate` [model](https://docs.victoriametrics.com/anomaly-detection/components/models/) states, so to migrate from v1.24.0-v1.24.1 it is recommended to drop the state |
| [v1.24.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1240) | [v1.24.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1241) | Fully Compatible | - |
| [v1.23.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1233) and earlier | [v1.24.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1240) | Fully Incompatible* | *As no state (prior to [v1.24.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1240)) existed, it was not saved (even if [on-disk mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode) was used). Also, see config breaking changes list in [stateless](https://docs.victoriametrics.com/anomaly-detection/migration/#stateless-mode) mode |

### Clearing State

For releases [v1.27.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270) and newer, the migration process is automatically handled by `vmanomaly` when started with `settings.restore_state: true`, so no manual intervention is required to clear existing state if incompatible.

However, for releases [v1.24.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1240) - [v1.26.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1262), to clear the existing state (if ended with `settings.restore_state: true`), please **manually delete** the existing state database and on-disk artifacts before starting the new version of `vmanomaly - either:
- Manually delete the content of `VMANOMALY_MODEL_DUMPS_DIR` / `VMANOMALY_DATA_DUMPS_DIR` folders or
- Set `settings.restore_state: false` in the config the first run of the new version, then stop `vmanomaly`, set back `settings.restore_state: true`, and restart `vmanomaly`.


### Stateless Mode

> Used if `settings.restore_state` is set to `false`. See argument details in the [configuration documentation](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration).

In stateless mode, the migration process is almost straightforward as there are no persistent states to manage. One may simply upgrade the `vmanomaly` service to the latest version and restart it, up to a slight change in the config .YAML files for backward-incompatible changes, see the list below.

**Breaking Changes**

- [v1.12.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1120) **ARIMA** model is removed from [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models); Action:  replace ARIMA by [Prophet](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or alternative seasonal models in `model(s)` section of your configuration files.

- [v1.9.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v190) The `sampling_period` parameter is now mandatory in `VmReader`. This change aims to clarify and standardize the frequency of input/output in `vmanomaly`, thereby reducing uncertainty and aligning with user expectations; Action: Add the `sampling_period` parameter to your `VmReader` configuration, e.g.:

  ```yaml
  reader:
    # Other VmReader settings...
    sampling_period: 1m
    ...
  ```