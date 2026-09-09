---
weight: 5
title: Migration
description: "Migration guide to the latest vmanomaly version."
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

For both upgrades and downgrades, assess compatibility at two levels:

- **Global compatibility**: Whether the target runtime can restore the persisted state as a whole. If globally incompatible, the state must be discarded and rebuilt. Global compatibility alone does not guarantee that every component's state can be reused.
- **Component compatibility**: Whether the target runtime can reuse individual components' persisted artifacts, such as model dumps. A component-specific incompatibility may require migration or discarding and rebuilding only the affected artifacts while retaining the remaining compatible state.

In the tables below, **source** means the version that wrote the persisted state and **target** means the version you intend to run, whether newer or older. Compatibility in one direction does not imply compatibility in the reverse direction.

> [!NOTE]
> Compatibility classifications and automated `--dryRun` or `/api/v1/compatibility` results cover vmanomaly-managed state formats, supported readers, and [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models). They do not validate custom model implementation or serialized dependencies, so compatibility is not guaranteed for [custom models](https://docs.victoriametrics.com/anomaly-detection/components/models/#custom-model-guide), even when the automated check reports `is_compatible: true`.
>
> When upgrading from v1.30.2 or earlier to v1.30.3 or newer, custom many-to-one models that rely only on `is_multivariate = True` must declare `topology = ModelTopology.MANY_TO_ONE` before upgrading. See the [custom model topology contract](https://docs.victoriametrics.com/anomaly-detection/components/models/#custom-model-guide).

> [!NOTE]
> Compatibility checks are directional and use the rules bundled with the running vmanomaly version. When planning a rollback, run `--dryRun` or `/api/v1/compatibility` from the current, newer binary because an older target binary cannot discover rules added after it was released.

Read each row as **source persisted state → target runtime**. The upgrade and downgrade tables are separate because compatibility is directional. Apply all relevant boundary rules when crossing several releases. A newer compatibility matrix can describe compatible formats even when an older executable does not recognize the newer provenance version; do not rewrite provenance to bypass that check.

<div class="collapse-group mb-3">

{{% collapse name="Upgrade compatibility" open=true %}}

| Source state | Target runtime | Action and compatibility boundary |
| --- | --- | --- |
| v1.30.4 | v1.30.5 | Reuse compatible built-in state. v1.30.5 extends the existing global compatibility chain; no new state-format migration is introduced by its UI or named-query workflow. |
| v1.30.0–v1.30.3 | v1.30.4–v1.30.5 | Reuse compatible built-in state. v1.30.4 restores legacy multivariate Temporal Envelope checkpoints correctly and advances state provenance after compatible upgrades. |
| v1.30.0–v1.30.2 | v1.30.3 | Multivariate Temporal Envelope checkpoints can fail during inference. Prefer upgrading directly to v1.30.4 or v1.30.5; otherwise discard and refit affected model state. |
| v1.29.1–v1.29.7 | v1.30.0–v1.30.5 | Existing built-in state remains compatible. Temporal Envelope was introduced in v1.30.0 and has no state from earlier releases. Review custom-model topology changes separately. |
| v1.28.x | v1.29.0 | Refit Prophet and Seasonal Quantile model dumps affected by the removed `pytz` dependency. Prefer v1.29.1 or newer, which restores this upgrade path. |
| v1.28.x | v1.29.1–v1.30.5 | The v1.29.0-only Prophet/Seasonal Quantile incompatibility does not propagate to these versions. Older component-specific restrictions still apply. |
| Before v1.28.0 | v1.28.0 or newer | Refit legacy `rolling_quantile` and `std` dumps if present: their class migration is a component boundary in the compatibility matrix. Earlier rolling implementations did not persist these artifacts by default. Other compatible state can be reused. |
| v1.25.3–v1.27.x | Later releases in the same global chain | Global state remains compatible; apply the model-specific boundaries above when crossing v1.28.0 or v1.29.0. |
| v1.25.1 | v1.25.2 | Reuse state within this database-format group. |
| v1.25.1–v1.25.2 | v1.25.3 or newer | Reinitialize state: v1.25.3 starts a new compatibility group and adds the model `forecast_at` layout. |
| v1.24.0–v1.25.0 | v1.25.1–v1.25.2 | Reinitialize state because v1.25.1 changes the metadata database format. |
| v1.24.0–v1.24.1 | v1.25.0 | Data layout and `quantile_online` / `isolation_forest_multivariate` dump formats change. Drop incompatible artifacts; for these older releases, a clean state is the recommended migration path. |
| v1.24.0 | v1.24.1 | Reuse compatible state. |
| v1.23.3 or earlier | v1.24.0 or newer | No restorable state was persisted by the earlier releases. Initialize state and review configuration changes below. |

{{% /collapse %}}

{{% collapse name="Downgrade compatibility" %}}

| Source state | Target runtime | Action and compatibility boundary |
| --- | --- | --- |
| v1.30.5 | v1.30.4 or earlier | The older shipped runtime does not know v1.30.5 provenance. Plan to reinitialize/refit state even where the newer matrix says the underlying formats are compatible. Do not edit the saved version. |
| v1.30.4 | v1.30.3 or earlier | The older shipped runtime does not recognize v1.30.4 provenance and drops persisted state. Reinitialize/refit on rollback. |
| v1.30.3 | v1.30.2 or earlier | Univariate and multivariate Temporal Envelope layouts changed in v1.30.3: discard and refit those models. This component restriction is in addition to the older runtime's provenance/global checks. |
| v1.30.0 or newer | Before v1.30.0 | Temporal Envelope is unavailable: replace unsupported model configuration and discard its model dumps. Check provenance and other component boundaries for the remaining state. |
| v1.29.1 or newer | v1.29.0 | The `pytz` restriction documented for v1.29.0 is upgrade-only, not a blanket downgrade restriction. The older runtime must still accept the actual saved version and model format. |
| v1.28.0 or newer | Before v1.28.0 | Online `rolling_quantile` / `std` dumps cannot be used by the older class implementations; discard them and configure a supported model. Check global provenance separately. |
| v1.25.3 or newer | v1.25.1–v1.25.2 | Crosses a global compatibility-group boundary: reinitialize state. |
| v1.25.2 | v1.25.1 | The metadata layout is shared; confirm the shipped target can recognize the recorded source version before attempting reuse. |
| v1.25.1–v1.25.2 | v1.24.0–v1.25.0 | The older metadata database layout is incompatible. Reinitialize state. |
| v1.25.0 | v1.24.0–v1.24.1 | Data and selected model dump layouts differ; a clean state is recommended. |
| v1.24.1 | v1.24.0 | Managed layouts are compatible; check recorded provenance against the shipped target. |
| Any stateful release | Before v1.24.0 | State restoration is unavailable; run with fresh state and configuration supported by the target. |

{{% /collapse %}}

</div>

Model availability is a separate check from state compatibility. ARIMA was removed in v1.12.0, before persisted-state restoration existed; remove or replace its configuration when upgrading across that boundary. The `mad` and `zscore` aliases redirect to online counterparts from v1.28.4. Prophet, Holt-Winters and Isolation Forest remain available for existing configurations in v1.30.5; planned deprecation is not removal. Always verify the target model catalog and the [configuration changes](#stateless-mode) below.

### Clearing State

For releases [v1.27.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1270) and newer, the migration process is automatically handled by `vmanomaly` when started with `settings.restore_state: true`, so no manual intervention is required to clear existing state if incompatible.

However, for releases [v1.24.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1240) - [v1.26.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1262), to clear the existing state (if ended with `settings.restore_state: true`), please **manually delete** the existing state database and on-disk artifacts before starting the new version of `vmanomaly - either:
- Manually delete the content of `VMANOMALY_MODEL_DUMPS_DIR` / `VMANOMALY_DATA_DUMPS_DIR` folders or
- Set `settings.restore_state: false` in the config the first run of the new version, then stop `vmanomaly`, set back `settings.restore_state: true`, and restart `vmanomaly`.


### Stateless Mode

> Used if `settings.restore_state` is set to `false`. See argument details in the [configuration documentation](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration).

In stateless mode, the migration process is almost straightforward as there are no persistent states to manage. One may simply upgrade the `vmanomaly` service to the latest version and restart it, up to a slight change in the config .YAML files for backward-incompatible changes, see the list below.

**Breaking Changes**

- [v1.12.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1120) **ARIMA** model is removed from [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models). Action: for vmanomaly v1.30.0 and newer, replace ARIMA with [Temporal Envelope](https://docs.victoriametrics.com/anomaly-detection/components/models/#temporal-envelope) {{% available_from "v1.30.0" anomaly %}}; for older releases, use another supported seasonal model in the `models` section of the configuration.

- [v1.9.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v190) The `sampling_period` parameter is now mandatory in `VmReader`. This change aims to clarify and standardize the frequency of input/output in `vmanomaly`, thereby reducing uncertainty and aligning with user expectations; Action: Add the `sampling_period` parameter to your `VmReader` configuration, e.g.:

  ```yaml
  reader:
    # Other VmReader settings...
    sampling_period: 1m
    ...
  ```
