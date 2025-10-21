---
weight: 6
title: CHANGELOG
menu:
  docs:
    identifier: "vmanomaly-changelog"
    parent: "anomaly-detection"
    weight: 6
tags:
  - metrics
  - enterprise
aliases:
- /anomaly-detection/CHANGELOG.html
---
Please find the changelog for VictoriaMetrics Anomaly Detection below.

## v1.26.2
Released: 2025-10-09

- IMPROVEMENT: Resolved an issue with readers ([VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [VLogsReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#victorialogs-reader)) connection pool size - which defaulted to max(10, `reader.queries` cardinality) - that could lead to warnings in logs when the number of queries exceeds 10, such as:
  ```shellhelp
  {timestamp} - urllib3.connectionpool - WARNING - Connection pool is full, discarding connection: {host}. Connection pool size: {N}
  ```
  This happened in scenarios with a large number of queries (e.g., in non-sharded deployments). Now the pool size is set dynamically to prevent such warnings and retain efficient connection reuse.

## v1.26.1  
Released: 2025-10-08

- IMPROVEMENT: Enriched lifecycle logs with the deterministic labelset hash for each query result (metric). This allows correlating model training, inference runs/skips, and on-disk artifacts presence or cleanup during incident triage.

## v1.26.0
Released: 2025-10-02

- FEATURE: Introduced vmui-like [UI](https://docs.victoriametrics.com/anomaly-detection/ui/) for `vmanomaly` service to simplify the configuration and backtesting of anomaly detection models before it goes to production. It provides an intuitive interface to finetune model configurations, visualize its predictions and anomaly scores, and perform backtesting on historical data. The UI is accessible via a web browser and can be run as a [standalone service](https://docs.victoriametrics.com/anomaly-detection/ui/#preset-usage) or [integrated with productionalized deployments](https://docs.victoriametrics.com/anomaly-detection/ui/#mixed-usage). For more details, refer to the [documentation](https://docs.victoriametrics.com/anomaly-detection/ui/).

- FEATURE: Added support for reading data from [VictoriaLogs stats queries](https://docs.victoriametrics.com/victorialogs/querying/#querying-log-range-stats) with `VLogsReader`. This reader allows quering and analyzing log data stored in VictoriaLogs, enabling anomaly detection on metrics generated from logs. It supports similar configuration options as `VmReader`, including `datasource_url`, `tenant_id`, `queries`, etc. For more details, refer to the [documentation](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vlogs-reader). It can be also used in [UI mode](https://docs.victoriametrics.com/anomaly-detection/ui/) for backtesting log-based anomaly detection configurations.

- IMPROVEMENT: Resolved the case in the [`IsolationForestModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#isolation-forest-multivariate) with `provide_series` common model [argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#provide-series) including `yhat.*` series (prediction and confidence boundaries), which are not produced by this model. Now config validation will fail with a clear error message if such series names are requested.

- BUGFIX: Recursive and shallow merging of the config files with mixed class names (`class` argument, with aliases like `zscore` and fully qualified names like `model.zscore.ZscoreModel`) now works as expected and is properly resolved to the same entity. Previously, this could lead to validation errors during service startup.

- BUGFIX: Fixed an issue with `anomaly_score_outside_data_range` [argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#score-outside-data-range) not being properly set for some models, resulting in default value (1.01) being used instead of user-defined override.

- BUGFIX: Fixed an issue with `decay` [parameter](https://docs.victoriametrics.com/anomaly-detection/components/models/#decay) not being properly applied to the global smoothing in the `OnlineQuantileModel` (when `seasonal_interval` is not set), resulted in no decay being applied (equivalent to `decay=1.0`).

## v1.25.3
Released: 2025-08-19

- FEATURE: Added forecasting capabilities to the [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) this allows users to generate *future* (point-wise and interval) predictions with offsets defined by `forecast_at` argument (e.g. `['1d', '1w']`) at *current* timestamp and store these in respective series, e.g. `yhat_1d`, `yhat_lower_1d`, `yhat_upper_1d`, etc. This feature is particularly useful for scenarios where future predictions are needed, such as capacity planning or trend analysis. See [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/#forecasting) for more details.

- IMPROVEMENT: Added `logger_levels` argument to `settings` [config section](https://docs.victoriametrics.com/anomaly-detection/components/settings/#logger-levels) to allow setting specific log levels for individual components. Useful for debugging specific components. For example, `logger_levels: { "reader.vm": "DEBUG" }` will set the log level for the `VmReader` component to `DEBUG`, while leaving other components at their default log levels. Also is supported in [hot reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) mode, allowing for dynamic log level changes without service restarts.

- IMPROVEMENT: Added logging of URLs used for querying VictoriaMetrics TSDB in [`VmReader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to ease the debugging of incomplete data retrieval, incorrect endpoints, or misconfigured tenant IDs. The URLs are logged at the `DEBUG` level, so you can override individual verbosity using the [settings.logger_levels](https://docs.victoriametrics.com/anomaly-detection/components/settings/#logger-levels) configuration.

- IMPROVEMENT: Added `offset` [argument](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to `VmReader` on reader and query levels to allow for flexible time offset adjustments in the reader. Useful for correcting for data collection delays. The `offset` can be specified as a string (e.g., "15s", "-20s") and will be applied to all queries processed by the reader. See [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/#using-offsets) for more details.

- BUGFIX: Resolved the issue where symlink-ed configuration files were not properly processed by [hot reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) mechanism, leading to the service not picking up changes made to the original files. Now it properly resolves symlinks and reloads the configuration when the original file is modified.

## v1.25.2
Released: 2025-07-30

- BUGFIX: Resolved inconsistent state between in-memory models and state database (if [stateful mode](https://docs.victoriametrics.com/anomaly-detection/components/settings/#stateful-mode) is enabled). This bug caused `Model instance not found` warnings during inference calls and prevented proper cleanup of stale models from disk. The fix also prevents state updates when operations are terminated mid-execution of scheduled fit/infer jobs.

- BUGFIX: Added explicit handling for inference calls on models that were deleted from disk by the time of their usage, but still referenced in the state database, preventing `'NoneType' object has no attribute 'infer'` rows in logs. Now a warning is logged and the inference call is skipped, which is expected behavior for deleted models.

## v1.25.1
Released: 2025-07-24

- IMPROVEMENT: Introduced `train_val_ratio` and `validation_scheme` options to `optimization_params` argument in [AutoTuned](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) model wrapper to cover more corner cases, such as using `validation_scheme: leaky`, to support setting `anomaly_percentage` ~ 0.0% (e.g., belief that training data has no anomalies at all) where the most deviational part of the data distribution reside at the very beginning of the time series. This is particularly useful for datasets with very few to no anomalies, where traditional validation methods may not be effective.

- IMPROVEMENT: Added [metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#startup-metrics) for convenient alerting on [hot reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) events:
  - `vmanomaly_config_last_reload_success_timestamp_seconds` - timestamp of the last successful hot reload
  - `vmanomaly_config_last_reload_successful` - gauge indicating if the last hot reload was successful (1) or not (0)
  - also renamed `vmanomaly_hot_reload_events_total` to `vmanomaly_config_reloads_total` and `vmanomaly_hot_reload_enabled` to `vmanomaly_config_reload_enabled` [metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#startup-metrics) to align with VictoriaMetrics' naming conventions.

- BUGFIX: Prevented the `vmanomaly` service from (gracefully) shutting down when a [hot reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) attempt fails if triggered again with erroneous config during currently processed *valid* hot reload.

- BUGFIX: Return earlier with a warning and increased `vmanomaly_model_runs_skipped` in case of empty dataframe received after filtering (valid values, seen timestamps) for specific timeseries at `infer` stage - instead of propagating the empty dataframe to the deeper level (model instance internals) where it otherwise lead to increase of `vmanomaly_model_run_errors` (e.g. seeing `Dataframe has no rows` Prophet error in logs).

- BUGFIX: Prevented `OneOffScheduler` and `BacktestingScheduler` [schedulers](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) from receiving no data (when [state restoration](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) is enabled). Now a warning is logged and such scheduler types are implicitly used without state restoration, which is expected behavior for these one-time-job schedulers.

- BUGFIX: Now the paths to artifact database (if [stateful mode](https://docs.victoriametrics.com/anomaly-detection/components/settings/#stateful-mode) is enabled) are properly resolved to absolute, preventing errors at initialization time (like `sqlalchemy.exc.OperationalError: (sqlite3.OperationalError) unable to open database file`) or warnings (like `SAWarning: fully NULL primary key identity cannot load any object.`).

## v1.25.0
Released: 2025-07-17

- FEATURE: Added [hot reload](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) support to automatically reload configurations on config files changes. It can be enabled via the `--watch` [CLI argument](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments) and allows for configuration updates without explicit service restarts. Please refer to the [hot-reload documentation](https://docs.victoriametrics.com/anomaly-detection/components/#hot-reload) for more details and examples on how to use it.

- FEATURE: Added an option to reference environment variables in [configuration files](https://docs.victoriametrics.com/anomaly-detection/components/) using scalar string placeholders `%{ENV_NAME}`. See the [environment variables](https://docs.victoriametrics.com/anomaly-detection/components/#environment-variables) section for more details and examples. This feature is particularly useful for managing sensitive information like API keys or database credentials while still making it accessible to the service.

- IMPROVEMENT: Added `iqr_threshold` to [OnlineQuantileModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) to refine the prediction boundaries without the need to manually adjusting `scale` [argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#scale). Best set as >= 2 and used with smaller, robust quantiles (e.g. `(0.25, 0.5, 0.75)`) to both reduce the impact of outliers on the prediction boundaries and increase the likelyhood of having "non-anomalous" data within updated boundaries.

- IMPROVEMENT: Fixed duplicated calls to VictoriaMetrics' in [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) for queries in `reader.queries` that are attached to multiple models in `models` [section](https://docs.victoriametrics.com/anomaly-detection/components/models/#queries) where previously, each model would independently fetch for the same query, leading to unnecessary load on the reader and VictoriaMetrics TSDB. Now, the reader will only be called once per unique (scheduler_alias, query_key) pair, and the results will be shared across all models that use the same query in the same scheduler.

## v1.24.1
Released: 2025-06-20

- BUGFIX: Resolved the issue first seen in [v1.23.0](#v1230) where some fit and infer jobs were silently skipped at task submission time (due to a bug in the new background scheduler behind [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler)) followed by similar warnings in the logs later on, such as:
  ```shellhelp
  2025-06-19 14:32:50,568 - apscheduler.executors.default - WARNING - Run time of job "{job_name}" (trigger: interval[1 day, 0:00:00], next run at: 2025-06-20 14:32:50 UTC)" was missed by 0:00:01.024753
  ```

- BUGFIX: Resolved the issue where `vmanomaly` service on [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where `start_from` argument was set and [state restoration](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) was enabled, didn't resume infer jobs after respective fitted models were restored from the previous run. This could lead to a situation where the service, *if restore happened in-between fit calls*, would not produce any anomaly scores and stay idle until the next `fit_every` happens, which is *expected in stateless mode*, but not in *stateful* mode with `restore_state` enabled.

## v1.24.0
Released: 2025-06-18

- FEATURE: Introduced stateful `vmanomaly` service with job persistence and state restoration capabilities. Added a new [`restore_state`](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) setting that enables the service to persist and restore its state between runs, including anomaly detection model instances and training data. This prevents unnecessary model refitting when restarting the service, significantly reducing startup time and computational overhead.

- IMPROVEMENT: More informative log messages for fit and infer stages and for sub-optimal configurations used in the [sharded mode](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability).

- BUGFIX: Now system interrupt signals are properly handled and lead to expected graceful shutdown if for some reason new background scheduler, introduced in [v1.23.0](#v1230) was already stopped in the middle of the fit or infer call. Previously, this could lead to a service crash with an unhandled exception.

## v1.23.3
Released: 2025-06-13

- IMPROVEMENT: Added backward-compatible single-dashed form support for `vmanomaly`'s [command-line arguments](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments) to enhance compatibility with VictoriaMetrics ecosystem and ease devOps experience. For example, `-license.forceOffline` can now be used in addition to `--license.forceOffline` - for the users who prefer the single-dash format or are accustomed to it from other VictoriaMetrics tools.

## v1.23.2
Released: 2025-06-09

- IMPROVEMENT: Increased convergence speed for [OnlineZScoreModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-z-score), [ZScoreModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#z-score), [MADModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#mad), and [OnlineMADModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-mad) models. Now it works better for tight optimization budgets (n_trials < 10, timeout < 1s)

- BUGFIX: Now mean and variance of [OnlineZScoreModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-z-score) with exponential `decay` < 1 [arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#decay) are properly calculated for unbiased predictions.

## v1.23.1
Released: 2025-06-08

- BUGFIX: In [sharding mode](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability) the corner case when shard number (`VMANOMALY_MEMBER_NUM`) is greater than the number of configured shards (`VMANOMALY_MEMBERS_COUNT`) is now properly handled.

- BUGFIX: In [sharding mode](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability), the corner case when the number of produced [sub-configurations](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#sub-configuration) is less than the number of configured shards (`VMANOMALY_MEMBERS_COUNT`) is now properly handled. Until config hot-reload is supported, such "idle" shards will be turned off with exit code 1 and respective critical message logged.

## v1.23.0
Released: 2025-06-05

> There is a known bug that can cause some fit and infer jobs to be silently skipped at task submission time (due to a bug in the new background scheduler behind [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler)) followed by similar warnings in the logs later on, such as:
>  ```shellhelp
>  2025-06-19 14:32:50,568 - apscheduler.executors.default - WARNING - Run time of job "{job_name}" (trigger: interval[1 day, 0:00:00], next run at: 2025-06-20 14:32:50 UTC)" was missed by 0:00:01.024753
>  ```
> Releases affected: [v1.23.0](#v1230) - [v1.23.3](#v1233).
> **The issue has been resolved in patch [v1.24.1](#v1241), upgrade is recommended.**

- FEATURE: Added `decay` [argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#decay) to [online models](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models). This parameters allows for newer data to be weighted more heavily in online models. By default this is set to 1 which means all data points are weighted the same to maintain backward compatibility with existing configs. The closer this value is to 0 the more important new data is.

- IMPROVEMENT: **Restored back parallelization** in the read/fit/infer pipeline, previously disabled in [v1.22.0](#v1220-experimental) due to deadlock issues. The new implementation prevents deadlocks, allowing to control the parallelization level via `n_workers` in [settings section](https://docs.victoriametrics.com/anomaly-detection/components/settings/). It's suggested to upgrade from [v1.22.0](#v1220) - [v1.22.1](#v1221) to this version to regain the performance benefits of parallel processing.

- IMPROVEMENT: Added `--dryRun` [argument](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments) to `vmanomaly` to enable dry run mode. This mode allows to validate configuration without executing any actual operations and doesn't require a license. It is particularly useful to test the configurations before deploying them in a production environment.

- IMPROVEMENT: Enhanced task scheduling to reduce locks between anomaly detection models' fit and inference calls, improving their concurrent performance.

- IMPROVEMENT: `min_dev_from_expected` model [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#minimal-deviation-from-expected) is now bi-directional, allowing you to set *different* thresholds for peaks and drops.

- BUGFIX: Now `clip_predictions` [model common arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#clip-predictions) is properly used with [online models](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models), ensuring that the predictions are clipped to the respective query's `data_range` values even if the model saw *less datapoints* than required `min_n_samples_seen_` to produce anomaly scores (e.g., when a new model instance was created during `infer` call for new timeseries not seen at training time).

## v1.22.1
Released: 2025-05-11

- FEATURE: Introduced a simplified backtesting mode for the [BacktestingScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#backtesting-scheduler) that treats your entire [`from`, `to`] (or [`from_iso`, `to_iso`]) range as an *inference* window and automatically generates the corresponding fit windows based on your `fit_window` setting. To enable it, set the `inference_only: true` flag in your BacktestingScheduler configuration.

- BUGFIX: Resolved a crash when running the [BacktestingScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#backtesting-scheduler) with `n_jobs` greater than 1.

- BUGFIX: Corrected the `start_from` logic in the [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) so that the *first* job now fires exactly at `start_from` (instead of occasionally adding `fit_every` to that time).

## v1.22.0-experimental
Released: 2025-04-11

**(Experimental Patch Release)**

> Important Notice - this patch disables parallelization to resolve rate but critical deadlock issue that completely halted the fit/infer pipeline (resulting in no anomaly scores, no model refits, and no log output) on multicore systems. Although this change improves resource usage by reducing peak-to-average RAM consumption, it incurs a 2–4x slowdown in fit/infer routines. We recommend upgrading only if your current deployments are experiencing deadlock-related outages. Please upgrade to [v1.23.0](#v1230) or newer for restored parallelization.

- BUGFIX: Resolved an intermittent deadlock in the fit/infer process that previously caused the service to freeze indefinitely, thereby preventing anomaly score production and model refits on multicore systems.

- BUGFIX: Fixed incorrect propagation of the `scale` model [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#scale) from the old format (single float) to the new format (list of 2 floats).

- IMPROVEMENT: Reduced the peak-to-average RAM usage for fit/infer calls from 2–2.5x to 1.1–1.3x, significantly lowering the risk of out-of-memory errors at startup.

## v1.21.0
Released: 2025-03-19

- FEATURE: Introduced [horizontal scalability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability) and [high availability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#high-availability) in `vmanomaly` service. Dedicated page can be found [here](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/).

## v1.20.1
Released: 2025-03-16

- BUGFIX: Resolved an issue in [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) introduced in [v1.18.7](#v1187) when inference is incorrectly skipped due to an outdated fit validation. The check mistakenly caused inference call skips in configurations where `fit_every` > `fit_window`. **Affected releases: [v1.18.7](#v1187) - [v1.20.0](#v1200)**. For `fit_every` > `fit_window` configurations we recommend upgrading to this patch release.

## v1.20.0
Released: 2025-03-03

> This release contains a bug introduced in [v1.18.7](#v1187) - [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- FEATURE: The `scale` argument is now a [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#scale), previously supported only by [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) and [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile). Additionally, `scale` is now **two-sided**, represented as `[scale_lb, scale_ub]`. The previous format (`scale: x`) remains supported and will be automatically converted to `scale: [x, x]`.
  
- FEATURE: Introduced a post-processing step to clip `yhat`, `yhat_lower`, and `yhat_upper` to the configured `data_range` [values](https://docs.victoriametrics.com/anomaly-detection/components/reader/) in `VmReader`, if defined. This feature is disabled by default for backward compatibility. It can be enabled for models that generate predictions and estimates, such as [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet), by setting the [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#clip-predictions) `clip_predictions` to `True`.

- IMPROVEMENT: Introduced the `anomaly_score_outside_data_range` [parameter](https://docs.victoriametrics.com/anomaly-detection/components/models/#score-outside-data-range) to allow overriding the default anomaly score (`1.01`) assigned when input values (`y`) fall outside the defined `data_range` (data domain violation). It improves flexibility for alerting rules and enables clearer visual distinction between different anomaly scenarios. Override can be configured at the **service level** (`settings`) or per **model instance** (`models.model_xxx`), with model-level values taking priority. If not explicitly set, the default anomaly score remains `1.01` for backward compatibility.

## v1.19.2
Released: 2025-01-27

> This release contains a bug introduced in [v1.18.7](#v1187) - [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- IMPROVEMENT: Added the `complete` option to the `--splitBy` argument in `config_splitter.py` [util](https://docs.victoriametrics.com/anomaly-detection/faq/#splitting-the-config). This allows splitting a parent configuration into the smallest possible sub-configurations, each containing exactly one scheduler, one model, and either one or multiple queries (depending on whether the model is [multivariate](https://docs.victoriametrics.com/anomaly-detection/components/models/#multivariate-models) or not).

- BUGFIX: Resolved an issue where duplicate log messages were generated during sub-config validation of the parent configuration.

- BUGFIX: Corrected usage of `AccountID` and `ProjectID` extracted from `tenant_id`, which are appended as labels `vm_account_id` and `vm_project_id`, respectively (previously swapped) by `VmReader` when using the per-query `tenant_id` feature. **This issue affected versions [v1.19.0](#v1190) and [v1.19.1](#v1191).**

- BUGFIX: Resolved an issue with the `VmReader` instance string representation that caused errors when `vmanomaly` was run with `--loggerLevel DEBUG`.

## v1.19.1
Released: 2025-01-21

> There is a known bug in [v1.19.0](#v1190) - the `AccountID` and `ProjectID` are swapped when they are extracted from the `tenant_id` argument in `VMReader`. This can cause correctly read results being written to the wrong tenant when using the per-query `tenant_id` feature with `AccountID` != `ProjectID`. Please update to patch [v1.19.2](#v1192), which resolves this issue.

> This release contains a bug introduced in [v1.18.7](#v1187) - [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- BUGFIX: Resolved writer warnings for configurations where `reader.tenant_id` equals `writer.tenant_id` and **is not** `multitenant`, as this is a valid setup. Enhanced tenant_id-related log messages across config validation, reader, and writer for improved clarity.

## v1.19.0
Released: 2025-01-20

> There is a known bug in [v1.19.0](#v1190) - the `AccountID` and `ProjectID` are swapped when they are extracted from the `tenant_id` argument in `VMReader`. This can cause correctly read results being written to the wrong tenant when using the per-query `tenant_id` feature with `AccountID` != `ProjectID`. Please update to patch [v1.19.2](#v1192), which resolves this issue.

> This release contains a bug introduced in [v1.18.7](#v1187) - [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- FEATURE: Added support for per-query `tenant_id` in the [`VmReader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). This allows overriding the reader-level `tenant_id` within a single global `vmanomaly` configuration on a *per-query* basis, enabling isolation of data for different tenants in separate queries when querying the [VictoriaMetrics cluster version](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/). For details, see the [documentation](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters).
- IMPROVEMEMT: Speedup the model infer stage on multicore systems.
- IMPROVEMEMT: Speedup the model fitting stage by 1.25-3x, depending on configuration complexity.
- IMPROVEMENT: Reduced service RAM usage by 5-10%, depending on configuration complexity.
- BUGFIX: Now [`VmReader`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) properly handles the cases where the number of queries processed in parallel (up to `reader.queries` cardinality) exceeds the default limit of 10 HTTP(S) connections, preventing potential data loss from discarded queries. The pool limit will automatically adjust to match `reader.queries` cardinality.
- BUGFIX: Corrected the construction of write endpoints for cluster VictoriaMetrics `url`s (`tenant_id` arg is set) in `monitoring.push` [section configurations](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters).

## v1.18.8
Released: 2024-12-03

> This release contains a bug introduced in [v1.18.7](#v1187) - [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- IMPROVEMENT: Added a `scale` parameter to [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet). It allows for proportional scaling of the confidence intervals generated by `interval_width`. If set > 1, it may help reducing false positives in scenarios where the data contains many sharp but expected seasonal peaks that may not be well captured by Prophet's seasonal [Fourier terms](https://en.wikipedia.org/wiki/Fourier_series).

- BUGFIX: Corrected an issue in [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) when using tz-aware mode with `tz_seasonalities` including `dow` (day of the week). Previously, _Sundays_ were incorrectly handled due to a mismatch between the weekday indices. This caused _Sundays_ to lack weekly seasonality features, defaulting to just averaged trends.

## v1.18.7
Released: 2024-12-02

> This release introduced a bug in [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) where configurations with `fit_every` > `fit_window` could cause inference to be skipped for |fit_every - fit_window| time, until the next `fit_every` call happens. For `fit_every` > `fit_window` configurations we recommend upgrading to [v1.20.1](#v1201), which resolves this issue.

- IMPROVEMENT: Introduced a new `push_frequency` parameter for the [monitoring.push component](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters), with a default value of 15m. This enhancement ensures better alignment with pull-based monitoring behavior and improves [self-monitoring experience](https://docs.victoriametrics.com/anomaly-detection/self-monitoring/) of `vmanomaly` in setups with infrequent schedules (e.g., rare `fit_every` or `infer_every` intervals) to deal with data staleness.

- BUGFIX: Fixed a bug, introduced in [v1.18.5](#v1185), that prevented the [monitoring.push component](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) from properly instantiating and pushing [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly).

## v1.18.6
Released: 2024-12-01

> Release [v1.18.5](#v1185) contained an issue that prevented the [monitoring.push component](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) from properly instantiating and pushing [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly). This issue has been resolved in patch [v1.18.7](#v1187), please update to apply the fix.

- BUGFIX: Assure proper validation of [BacktestingScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#backtesting-scheduler) arguments, if specified in ISO-8601 format, preventing service crashes due to validation errors.


## v1.18.5
Released: 2024-11-27

> This release contained an issue that prevented the [monitoring.push component](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) from properly instantiating and pushing [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly). This issue has been resolved in patch [v1.18.7](#v1187), please update to apply the fix.

- IMPROVEMENT: Introduced the ability to run `vmanomaly` using a configuration directory. This enhancement allows users to recursively merge multiple full configuration files (previously limited to merging specific sections, such as `reader`) and execute a single instance of the service with the combined configuration.
- IMPROVEMENT: Added a new utility, `config_splitter.py`, to streamline the process of splitting a single configuration file into multiple standalone configurations. The configurations are split by specified entities like `schedulers`, `models`, `queries` or `extra_filters`. The split configurations can be saved to a designated directory. It simplifies scaling `vmanomaly` and enhances user experience by automating the process of separating config files so they can be run on separate instances of vmanomaly. For more details, refer to [this section](https://docs.victoriametrics.com/anomaly-detection/faq/#splitting-the-config).
- IMPROVEMENT: Introduced the ability to configure the [`PeriodicScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler) to start at a specific time using the `start_from` and `tz` parameters. The `start_from` parameter accepts either `HH:MM` or [ISO 8601 formats](https://en.wikipedia.org/wiki/ISO_8601), with `tz` defaulting to `UTC`. If `start_from` is in the past, the next valid start time is automatically calculated based on the `fit_every` interval.

## v1.18.4
Released: 2024-11-18

- IMPROVEMENT: Introduced [self-monitoring guide](https://docs.victoriametrics.com/anomaly-detection/self-monitoring/) for `vmanomaly`. Added metrics for total RAM `vmanomaly_available_memory_bytes` and the number of logical CPU cores `vmanomaly_cpu_cores_available` to the [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly).

## v1.18.3
Released: 2024-11-14

- BUGFIX: This patch release resolves an issue that could cause a service crash when parallelizing data processing with `VmReader`. Affected releases: [v1.18.1](#v1181) - [v1.18.2](#v1182).

## v1.18.2
Released: 2024-11-13

> In release [v1.18.1](#v1181), an issue was identified that could lead to a service crash during parallelized data processing with [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). Please update to patch [v1.18.3](#v1183), which resolves this issue.

- IMPROVEMENT: Enhanced the flexibility of the [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) for tz-aware data (`tz_aware = True`). The `tz_seasonalities` argument has been reformatted to align with the structure of the existing `seasonalities` argument. For more details, refer to the [model section here](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet). Additionally, tz-aware support for `ProphetModel` has been added to [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) model wrapper. This feature is automatically enabled if the data is timezone-aware and its timezone is not set to the default ('UTC'), otherwise default timezone-free optimization flow will be used.

## v1.18.1
Released: 2024-11-12

> In release [v1.18.1](#v1181), an issue was identified that could lead to a service crash during parallelized data processing with [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). Please update to patch [v1.18.3](#v1183), which resolves this issue.

- IMPROVEMENT: Added a [reader-level](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) `data_range` argument, allowing users to define a default *valid* data range for all input queries in `queries`. Individual queries can still override this default with their own `data_range` if needed.
- IMPROVEMENT: Added the `url` label to enhance labelset consistency across [self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) in both [reader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics) components. Metrics affected:
  - `vmanomaly_reader_received_bytes`
  - `vmanomaly_reader_response_parsing_seconds`
  - `vmanomaly_reader_timeseries_received`
  - `vmanomaly_reader_datapoints_received`
  - `vmanomaly_writer_request_serialize_seconds`
  - `vmanomaly_writer_datapoints_sent`
  - `vmanomaly_writer_timeseries_sent`

- BUGFIX: Resolved an issue where [rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models) incorrectly set their last seen `infer` timestamp during *first* `fit_infer` call, resulting in output being produced for *every datapoint* within the `fit_window` on its *first invocation*.
- BUGFIX: Resolved an issue in multi-[scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) configurations where [self-monitoring metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) values were overwriting each other.
- BUGFIX: Resolved an issue causing incorrect `query_key` label values in the `vmanomaly_model_datapoints_produced` [self-monitoring metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics) for [univariate models](https://docs.victoriametrics.com/anomaly-detection/components/models/#univariate-models).
- BUGFIX: Resolved an issue that caused the `vmanomaly_model_runs` [self-monitoring metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics) to miss increments for [rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models).
- BUGFIX: Aligned the calculations of `vmanomaly_model_datapoints_accepted` and `vmanomaly_model_datapoints_produced` [self-monitoring model metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics) across all stages (`fit`, `infer`, and `fit_infer`) for consistency.

## v1.18.0
Released: 2024-10-28

- FEATURE: Introduced timezone-aware support in `VmReader` for accurate seasonality modeling, especially during DST shifts. A new `tz` argument enables timezone offset management at both global and [query-specific levels](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters).
  - Enhanced [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) with a `tz_aware` argument (combined with `tz_seasonalities` and `tz_use_cyclical_encoding`) for timezone-aware timestamps. This addresses a [limitation in Prophet's native design](https://github.com/facebook/prophet/blob/dc1df4cb23a150e14858afb34c9442401c0eb2fc/python/prophet/forecaster.py#L288) that doesn't allow timezone-aware and DST-aware seasonality.

- IMPROVEMENT: Enhanced error handling in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to provide clearer diagnostics and broader coverage.

- BUGFIX: Updated `vmanomaly_version_info` and `vmanomaly_ui_version_info` gauges to correctly set the version label value based on image tags.
- BUGFIX: The `n_samples_seen_` attribute now properly resets to 0 with each new `fit` call in [online model](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) classes ([`OnlineMADModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-mad) and [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile)), ensuring accurate tracking of processed sample count.

## v1.17.2
Released: 2024-10-22

- IMPROVEMENT: Added `vmanomaly_version_info` (service) and `vmanomaly_ui_version_info` (vmui) gauges to self-monitoring metrics.
- IMPROVEMENT: Added `instance` and `job` labels to [pushed](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model) metrics so they have the same labels as vmanomaly metrics that are [pulled](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#pull-model)/scraped. Metric labels can be customized via the [`extra_labels` argument](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters). By default job label will be `vmanomaly` and the instance label will be `f'{hostname}:{vmanomaly_port}`. See [monitoring.push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) for examples and details.
- IMPROVEMENT: Added a subsection to [monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#logs-generated-by-vmanomaly) page with detailed per-component service logs, including reader and writer logs, error handling, metrics updates, and multi-tenancy warnings.
- IMPROVEMENT: Added a new [Command-line arguments](https://docs.victoriametrics.com/anomaly-detection/quickstart/#command-line-arguments) subsection to the [Quickstart guide](https://docs.victoriametrics.com/anomaly-detection/quickstart/), providing details on available options for configuring `vmanomaly`.


## v1.17.1
Released: 2024-10-18

- BUGFIX: [Prophet models](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) no longer fail to train on *constant* data, data consisting of the same value and no variation across time. The bug prevented the `fit` stage from completing successfully, resulting in the model instance not being stored in the model registry, after automated model cleanup was added in [v1.17.0](#v1170).

## v1.17.0
Released: 2024-10-17

- FEATURE: Added `max_points_per_query` (global and [query-specific](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters)) [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) arg to control query chunking. This overrides how `search.maxPointsPerTimeseries` flag (introduced in [v1.14.1](#v1141)) is used in `vmanomaly` for splitting long `fit_window` queries into smaller sub-intervals. This helps users avoid hitting the `search.maxQueryDuration` limit for individual queries by distributing initial query across multiple subquery requests with minimal overhead.

- IMPROVEMENT: Enhanced the [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) metrics for consistency across the components. Key changes include:
  - Converted several [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) metrics from `Summary` to `Histogram` to enable quantile calculation. This addresses the limitation of the `prometheus_client`'s [Summary](https://prometheus.github.io/client_python/instrumenting/summary/) implementation, which does not support quantiles. The change ensures metrics are more informative for performance analysis. Affected metrics are:
    - `vmanomaly_reader_request_duration_seconds` ([VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics))
    - `vmanomaly_reader_response_parsing_seconds` ([VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics))
    - `vmanomaly_writer_request_duration_seconds` ([VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics))
    - `vmanomaly_writer_request_serialize_seconds` ([VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics))
  - Added a `query_key` label to the `vmanomaly_reader_response_parsing_seconds` [metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) to provide finer granularity in tracking the performance of individual queries. This metric has also been switched from `Summary` to `Histogram` to align with the other metrics and support quantile calculations.
  - Added `preset` and `scheduler_alias` keys to [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) and [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#writer-behaviour-metrics) metrics for consistency in multi-[scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) setups.
  - Renamed [Counters](https://prometheus.io/docs/concepts/metric_types/#counter) `vmanomaly_reader_response_count` to `vmanomaly_reader_responses` and `vmanomaly_writer_response_count` to `vmanomaly_writer_responses`.
  - Updated [docs](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) for better clarity.

- IMPROVEMENT: Accelerated performance of model fitting stages on multicore systems.
- IMPROVEMENT: Optimized query handling in multi-[scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) setups by filtering [queries](https://docs.victoriametrics.com/anomaly-detection/components/models/#queries) for each scheduler based on model requirements. This reduces unnecessary data fetching from VictoriaMetrics, ensuring only relevant queries are processed by the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), leading to better performance and efficiency of configs with multiple active schedulers.

- IMPROVEMENT: Implemented automatic cleanup of files in subdirectories within `/tmp` ([generated by the Stan backend](https://mc-stan.org/cmdstanpy/users-guide/outputs) when utilizing [Prophet](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) models) after each `fit` operation. This prevents the accumulation of unused data over time in `/tmp`, addressing a potential issue where these files would only be deleted upon termination of the current Python session or service, leading to uncontrolled disk growth.

- BUGFIX: Re-enable the `vmanomaly_reader_response_count` (now called `vmanomaly_reader_responses`) self-monitoring [metric](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics) for the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), which was unintentionally disabled in previous releases and now updates correctly as intended.

## v1.16.3
Released: 2024-10-08
- IMPROVEMENT: Added `tls_cert_file` and `tls_key_file` arguments to support mTLS (mutual TLS) in `vmanomaly` components. This enhancement applies to the following components: [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer), and [Monitoring/Push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters). You can also use these arguments in conjunction with `verify_tls` when it is set as a path to a custom CA certificate file.

## v1.16.2
Released: 2024-10-06
- FEATURE: Added support for `multitenant` value in `tenant_id` arg to enable querying across multiple tenants in [VictoriaMetrics cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) (option available from [v1.104.0](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy-via-labels)):
  - Applied when reading input data from `vmselect` via the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader).
  - Applied when writing generated results through `vminsert` via the [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer).
  - For more details, refer to the `tenant_id` arg description in the documentation of the mentioned components.

- BUGFIX: Resolved an issue with handling an empty `preset` value (e.g., `preset: ""`) that was preventing the [default helm chart](https://github.com/VictoriaMetrics/helm-charts/blob/7f5a2c00b14c2c088d7d8d8bcee7a440a5ff11c6/charts/victoria-metrics-anomaly/values.yaml#L139) from being deployed.

## v1.16.1
Released: 2024-10-02
- BUGFIX: This patch release prevents the service from crashing by rolling back the version of a third-party dependency. Affected releases: [v1.16.0](#v1160).

## v1.16.0
Released: 2024-10-01

> A bug was discovered in this release that causes the service to crash. Please use the patch [v1.16.1](#v1161) to resolve this issue.

- FEATURE: Introduced data dumps to a host filesystem for [VmReader](https://docs.victoriametrics.com/anomaly-detection/#vm-reader).  Resource-intensive setups (multiple queries returning many metrics, bigger `fit_window` arg) will have RAM consumption reduced during fit calls.
- IMPROVEMENT: Added a `groupby` argument for logical grouping in [multivariate models](https://docs.victoriametrics.com/anomaly-detection/components/models/#multivariate-models). When specified, a separate multivariate model is trained for each unique combination of label values in the `groupby` columns. For example, to perform multivariate anomaly detection on metrics at the machine level without cross-entity interference, you can use `groupby: [host]` or `groupby: [instance]`, ensuring one model per entity being trained (e.g., per host). Please find more details [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#group-by).
- IMPROVEMENT: Improved performance of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) on multicore instances for reading and data processing.
- IMPROVEMENT: Introduced new CLI argument aliases to enhance compatibility with [Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/README.md) (i.e. using secrets) and better align with [VictoriaMetrics flags](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#list-of-command-line-flags):
  - `--licenseFile` as an alias for `--license-file`
  - `--license.forceOffline` as an alias for `--license-verify-offline`
  - `--loggerLevel` as an alias for `--log-level`
  - The previous argument format is retained for backward compatibility.

- BUGFIX: The `provide_series` [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#provide-series) now correctly filters the written time series in the [IsolationForestMultivariate](https://docs.victoriametrics.com/anomaly-detection/components/models/#isolation-forest-multivariate) model.

## v1.15.9
Released: 2024-08-27
- IMPROVEMENT: Added support for bearer token authentication in `push` mode within the [self-monitoring configuration section](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters).

## v1.15.8
Released: 2024-08-27
- BUGFIX: Made minor adjustments to how the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) handle bearer tokens across different modes.

## v1.15.7
Released: 2024-08-27
- BUGFIX: Made minor adjustments to how the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) handle bearer tokens across different modes.

## v1.15.6
Released: 2024-08-26
- IMPROVEMENT: Introduced the `bearer_token_file` argument to the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) components to enhance secret management.


## v1.15.5
Released: 2024-08-19
- BUGFIX: following [v1.15.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1152) online model enhancement, now `data_range` parameter is correctly initialized for online models, created (for new time series returned by particular query) during `infer` calls. 

## v1.15.4
Released: 2024-08-15
- IMPROVEMENT: better config handling of [writer](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/) sections if using `vmanomaly` with [helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).

## v1.15.3
Released: 2024-08-14
- IMPROVEMENT: better config handling of `reader` [section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) if using `vmanomaly` with [helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).

## v1.15.2
Released: 2024-08-13
- IMPROVEMENT: Enhanced [online models](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) (e.g., [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile)) to automatically create model instances for unseen time series during `infer` calls, eliminating the need to wait for the next `fit` call. This ensures no inferences are skipped **when using online models**.
- BUGFIX: Corrected an issue with the [`OnlineMADModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-mad) to ensure proper functionality when used in combination with [on-disk model dump mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).
- BUGFIX: Addressed numerical instability in the [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) when `use_transform` is set to `True`.
- BUGFIX: Resolved a logging issue that could cause a `RuntimeError: reentrant call inside <_io.BufferedWriter name='<stderr>'>` when a termination event was received.

## v1.15.1
Released: 2024-08-10
- FEATURE: Introduced backward-compatible `data_range` [query-specific parameter](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters) to the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). It enables the definition of **valid** data ranges for input per individual query in `queries`, resulting in:
  - **High anomaly scores** (>1) when the *data falls outside the expected range*, indicating a data constraint violation.
  - **Lowest anomaly scores** (=0) when the *model's predictions (`yhat`) fall outside the expected range*, signaling uncertain predictions.
  - For more details, please refer to the [documentation](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters).

- IMPROVEMENT: Added `latency_offset` argument to the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to override the default `-search.latencyOffset` [flag of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/#list-of-command-line-flags) (30s). The default value is set to 1ms, which should help in cases where `sampling_frequency` is low (10-60s) and `sampling_frequency` equals `infer_every` in the [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler). This prevents users from receiving `service - WARNING - [Scheduler [scheduler_alias]] No data available for inference.` warnings in logs and allows for consecutive `infer` calls without gaps. To restore the backward compatible behavior, set it equal to your `-search.latencyOffset` value in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) config section.

- BUGFIX: Ensure the `use_transform` argument of the [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) functions as intended.
- BUGFIX: Add a docstring for `query_from_last_seen_timestamp` arg of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader).


## v1.15.0
Released: 2024-08-06
- FEATURE: Introduced models that support [online learning](https://en.wikipedia.org/wiki/Online_machine_learning) for stream-like input. These models significantly reduce the amount of data required for the initial fit stage. For example, they enable reducing `fit_every` from **weeks to hours** and increasing `fit_every` from **hours to weeks** in the [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler), significantly reducing the **peak amount** of data queried from VictoriaMetrics during `fit` stages. The next models were added:
  - [`OnlineZscoreModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-z-score) - online version of existing [Z-score](https://docs.victoriametrics.com/anomaly-detection/components/models/#z-score) implementation with the same exact behavior.
  - [`OnlineMADModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-mad) - online version of existing [MADModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#mad-median-absolute-deviation) implementation with *approximate* behavior, based on [t-digests](https://www.sciencedirect.com/science/article/pii/S2665963820300403) for online quantile estimation.
  - [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) - online quantile model, that supports custom ranges for seasonality estimation to cover more complex data patterns.
  - Find out more about online models specifics in [correspondent section](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models).

- FEATURE: Introduced the `optimized_business_params` key (list of strings) to the [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) `optimization_params`. This allows particular business-specific parameters such as [`detection_direction`](https://docs.victoriametrics.com/anomaly-detection/components/models/#detection-direction) and [`min_dev_from_expected`](https://docs.victoriametrics.com/anomaly-detection/components/models/#minimal-deviation-from-expected) to remain **unchanged during optimizations, retaining their default values**.
- IMPROVEMENT: Optimized the [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) model logic to minimize deviations from the expected `anomaly_percentage` specified in the configuration and the detected percentage in the data, while also reducing discrepancies between the actual values (`y`) and the predictions (`yhat`).
- IMPROVEMENT: Allow [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) to fit with multiple seasonalities when used in [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) mode. 

## v1.14.2
Released: 2024-07-26
- BUGFIX: Patch a bug introduced in [v1.14.1](#v1141), causing `vmanomaly` to crash in `preset` [mode](https://docs.victoriametrics.com/anomaly-detection/presets/).

## v1.14.1
Released: 2024-07-26
- FEATURE: Allow to process larger data chunks in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) that exceed `-search.maxPointsPerTimeseries` [constraint in VictoriaMetrics](https://docs.victoriametrics.com/#resource-usage-limits) by splitting the range and sending multiple requests. A warning is printed in logs, suggesting reducing the range or step, or increasing `search.maxPointsPerTimeseries` constraint in VictoriaMetrics, which is still a recommended option.
- FEATURE: Backward-compatible redesign of [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) arg of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). Old format of `{q_alias1: q_expr1, q_alias2: q_expr2, ...}` will be implicitly converted to a new one with a warning raised in logs. New format allows to specify per-query parameters, like `step` to reduce amount of data read from VictoriaMetrics TSDB and to allow config flexibility. Find out more in [Per-query parameters section of VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters).

- IMPROVEMENT: Added multi-platform builds for `linux/amd64` and `linux/arm64` architectures.

## v1.13.3
Released: 2024-07-17
- BUGFIX: now validation of `args` argument for [`HoltWinters`](https://docs.victoriametrics.com/anomaly-detection/components/models/#holt-winters) model works properly.

## v1.13.2
Released: 2024-07-15
- IMPROVEMENT: update `node-exporter` [preset](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter) to reduce [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive)
- BUGFIX: add `verify_tls` arg for [`push`](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) monitoring section. Also, `verify_tls` is now correctly used in [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer).
- BUGFIX: now [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) model wrapper works correctly in [on-disk model storage mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).
- BUGFIX: now [rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models), like [`RollingQuantile`](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-quantile) are properly handled in [One-off scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#oneoff-scheduler), when wrapped in [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned)

## v1.13.0
Released: 2024-06-11
- FEATURE: Introduced `preset` [mode to run vmanomaly service](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter) with minimal user input and on widely-known metrics, like those produced by [`node_exporter`](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter).
- FEATURE: Introduced `min_dev_from_expected` [model common arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#minimal-deviation-from-expected), aimed at **reducing [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive)** in scenarios where deviations between the real value `y` and the expected value `yhat` are **relatively** high and may cause models to generate high [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score). However, these deviations are not significant enough in **absolute values** to be considered anomalies based on domain knowledge.
- FEATURE: Introduced `detection_direction` [model common arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#detection-direction), enabling domain-driven anomaly detection strategies. Configure models to identify anomalies occurring *above, below, or in both directions* relative to the expected values.
- FEATURE: add `n_jobs` arg to [`BacktestingScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#backtesting-scheduler) to allow *proportionally faster (yet more resource-intensive)* evaluations of a config on historical data. Default value is 1, that implies *sequential* execution.
- FEATURE: allow anomaly detection models to be dumped to a host filesystem after `fit` stage (instead of in-memory). Resource-intensive setups (many models, many metrics, bigger [`fit_window` arg](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler-config-example)) and/or 3rd-party models that store fit data (like [ProphetModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or [HoltWinters](https://docs.victoriametrics.com/anomaly-detection/components/models/#holt-winters)) will have RAM consumption greatly reduced at a cost of slightly slower `infer` stage. Please find how to enable it [here](https://docs.victoriametrics.com/anomaly-detection/faq/#resource-consumption-of-vmanomaly)
- IMPROVEMENT: Reduced the resource used for each fitted [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) by up to 6 times. This includes both RAM for in-memory models and disk space for on-disk models storage. For more details, refer to [this discussion on Facebook's Prophet](https://github.com/facebook/prophet/issues/1159#issuecomment-537415637).
- IMPROVEMENT: now config [components](https://docs.victoriametrics.com/anomaly-detection/components/) class can be referenced by a short alias instead of a full class path - i.e. `model.zscore.ZscoreModel` becomes `zscore`, `reader.vm.VmReader` becomes `vm`, `scheduler.periodic.PeriodicScheduler` becomes `periodic`, etc.
- BUGFIX: if using multi-scheduler setup (introduced in [v1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110)), prevent schedulers (and correspondent services) that are not attached to any model (so neither found in ['schedulers'  arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#schedulers) nor left blank in `model` section) from being spawn, causing resource overhead and slight interference with existing ones.
- BUGFIX: set random seed for [ProphetModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) to assure uncertainty estimates (like `yhat_lower`, `yhat_upper`) and dependent series (like `anomaly_score`), produced during `.infer()` calls are always deterministic given the same input. See [initial issue](https://github.com/facebook/prophet/issues/1124) for the details.
- BUGFIX: prevent *orphan* queries (that are not attached to any model or scheduler) found in `queries` arg of [Reader config section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to be fetched from VictoriaMetrics TSDB, avoiding redundant data processing. A warning will be logged, if such queries exist in a parsed config.

## v1.12.0
Released: 2024-03-31
- FEATURE: Introduction of `AutoTunedModel` model class to optimize any [built-in model](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models) on data during `fit` phase. Specify as little as `anomaly_percentage` param from `(0, 0.5)` interval and `tuned_model_class` (i.e. [`model.zscore.ZscoreModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#z-score)) to get it working with best settings that match your data. See details [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned).
<!--
- FEATURE: Preset support enablement. From now users will be able to specify only a few parameters (like `datasource_url`) + a new (backward-compatible) `preset: preset_name` field in a config file and get a service run with **predefined queries, scheduling and models**. Also, now preset assets (guide, configs, dashboards) will be available at `:8490/presets` endpoint.
-->
- IMPROVEMENT: Better logging of model lifecycle (fit/infer stages).
- IMPROVEMENT: Introduce `provide_series` arg to all the [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models) to define what output fields to generate for writing (i.e. `provide_series: ['anomaly_score']` means only scores are being produced)
- BUGFIX: [Self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics) are now aggregated to `queries` aliases level (not to label sets of individual timeseries) and aligned with [reader, writer and model sections](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) description , so `/metrics` endpoint holds only necessary information for scraping.
- BUGFIX: Self-monitoring metric `vmanomaly_models_active` now has additional labels `model_alias`, `scheduler_alias`, `preset` to align with model-centric [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics).
- IMPROVEMENT: Add possibility to use temporal information in [IsolationForest models](https://docs.victoriametrics.com/anomaly-detection/components/models/#isolation-forest-multivariate) via [cyclical encoding](https://towardsdatascience.com/cyclical-features-encoding-its-about-time-ce23581845ca). This is particularly helpful to detect multivariate [seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality)-dependant anomalies.
- BREAKING CHANGE: **ARIMA** model is removed from [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models). For affected users, it is suggested to replace ARIMA by [Prophet](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or [Holt-Winters](https://docs.victoriametrics.com/anomaly-detection/components/models/#holt-winters).

## v1.11.0
Released: 2024-02-22
- FEATURE: Multi-scheduler support. Now users can use multiple [model specs](https://docs.victoriametrics.com/anomaly-detection/components/models/) in a single config (via aliasing), each spec can be run with its own (even multiple) [schedulers](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/).
  - Introduction of `schedulers` arg in model spec:
    - It allows each model to be managed by 1 (or more) schedulers, so overall resource usage is optimized and flexibility is preserved. 
    - Passing an empty list or not specifying this param implies that each model is run in **all** the schedulers, which is a backward-compatible behavior.
    - Please find more details in docs on [Model section](https://docs.victoriametrics.com/anomaly-detection/components/models/#schedulers)
- DEPRECATION: slight refactor of a scheduler config section 
  - Now schedulers are passed as a mapping of `scheduler_alias: scheduler_spec` under [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) sections. Using old format (< [1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110)) will produce warnings for now and will be removed in future versions.
- DEPRECATION: The `--watch` CLI option for config file reloads is deprecated and will be ignored in the future.

## v1.10.0
Released: 2024-02-15
- FEATURE: Multi-model support. Now users can specify multiple [model specs](https://docs.victoriametrics.com/anomaly-detection/components/models/) in a single config (via aliasing), as well as to reference what [queries from VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#config-parameters) it should be run on.
  - Introduction of `queries` arg in model spec:
    - It allows the model to be executed only on a particular query subset from `reader` section. 
    - Passing an empty list or not specifying this param implies that each model is run on results from **all** queries, which is a backward-compatible behavior.
    - Please find more details in docs on [Model section](https://docs.victoriametrics.com/anomaly-detection/components/models/#queries)

- DEPRECATION: slight refactor of a model config section 
  - Now models are passed as a mapping of `model_alias: model_spec` under [model](https://docs.victoriametrics.com/anomaly-detection/components/models/) sections. Using old format (<= [1.9.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v192)) will produce warnings for now and will be removed in future versions.
  - Please find more details in docs on [Model section](https://docs.victoriametrics.com/anomaly-detection/components/models/)
- IMPROVEMENT: now logs from [`monitoring.pull`](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#monitoring-section-config-example) GET requests to `/metrics` endpoint are shown only in DEBUG mode
- IMPROVEMENT: labelset for multivariate models is deduplicated and cleaned, resulting in better UX

> These updates support more flexible setup and effective resource management in service, as now it's not longer needed to spawn several instances of `vmanomaly` to split queries/models context across.


## v1.9.2
Released: 2024-01-29
- BUGFIX: now multivariate models (like [`IsolationForestMultivariateModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#isolation-foresthttpsenwikipediaorgwikiisolation_forest-multivariate)) are properly handled throughout fit/infer phases.


## v1.9.1
Released: 2024-01-27
- IMPROVEMENT: Updated the offline license verification backbone to mitigate a critical vulnerability identified in the [`ecdsa`](https://pypi.org/project/ecdsa/) library, ensuring enhanced security despite initial non-impact.
- IMPROVEMENT: bump 3rd-party dependencies for Python 3.12.1


## v1.9.0
Released: 2024-01-26
- BUGFIX: The `query_from_last_seen_timestamp` internal logic in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), first introduced in [v1.5.1](#v151), now functions correctly. This fix ensures that the input data shape remains consistent for subsequent `fit`-based model calls in the service.
- BREAKING CHANGE: The `sampling_period` parameter is now mandatory in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). This change aims to clarify and standardize the frequency of input/output in `vmanomaly`, thereby reducing uncertainty and aligning with user expectations.

> The majority of users, who have been proactively specifying the `sampling_period` parameter in their configurations, will experience no disruption from this update. This transition formalizes a practice that was already prevalent and expected among our user base.


## v1.8.0
Released: 2024-01-15
- FEATURE: Added Univariate [MAD (median absolute deviation)](https://docs.victoriametrics.com/anomaly-detection/components/models/#mad-median-absolute-deviation) model support.
- IMPROVEMENT: Update Python to 3.12.1 and all the dependencies.
- IMPROVEMENT: Don't check /health endpoint, check the real /query_range or /import endpoints directly. Users kept getting problems with /health.
- DEPRECATION: "health_path" param is deprecated and doesn't do anything in config ([reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer), [monitoring.push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters)).


## v1.7.2
Released: 2023-12-21
- BUGFIX: fit/infer calls are now skipped if we have insufficient *valid* data to run on.
- BUGFIX: proper handling of `inf` and `NaN` in fit/infer calls.
- FEATURE: add counter of skipped model runs `vmanomaly_model_runs_skipped` to healthcheck metrics.
- FEATURE: add exponential retries wrapper to VmReader's `read_metrics()`.
- FEATURE: add `BacktestingScheduler` for consecutive retrospective fit/infer calls.
- FEATURE: add improved & numerically stable anomaly scores.
- IMPROVEMENT: add full config validation. The probability of getting errors in later stages (say, model fit) is greatly reduced now. All the config validation errors that needs to be fixed are now a part of logging.
  > This is an backward-incompatible change, as `model` config section now expects key-value args for internal model defined in nested `args`.
- IMPROVEMENT: add explicit support of `gzip`-ed responses from vmselect in VmReader.


## v1.6.0
Released: 2023-10-30
- IMPROVEMENT: 
  - now all the produced healthcheck metrics have `vmanomaly_` prefix for easier accessing.
  - updated docs for monitoring.
  > This is an backward-incompatible change, as metric names will be changed, resulting in new metrics creation, i.e. `model_datapoints_produced` will become `vmanomaly_model_datapoints_produced`
- IMPROVEMENT: Set default value for `--log_level` from `DEBUG` to `INFO` to reduce logs verbosity.
- IMPROVEMENT: Add alias `--log-level` to `--log_level`.
- FEATURE: Added `extra_filters` parameter to reader. It allows to apply global filters to all queries.
- FEATURE: Added `verify_tls` parameter to reader and writer. It allows to disable TLS verification for remote endpoint.
- FEATURE: Added `bearer_token` parameter to reader and writer. It allows to pass bearer token for remote endpoint for authentication.
- BUGFIX: Fixed passing `workers` parameter for reader. Previously it would throw a runtime error if `workers` was specified.

## v1.5.1
Released: 2023-09-18
- IMPROVEMENT: Infer from the latest seen datapoint for each query. Handles the case datapoints arrive late. 


## v1.5.0
Released: 2023-08-11
- FEATURE: add `--license` and `--license-file` command-line flags for license code verification. 
- IMPROVEMENT: Updated Python to 3.11.4 and updated dependencies.
- IMPROVEMENT: Guide documentation for Custom Model usage.


## v1.4.2
Released: 2023-06-09
- BUGFIX: Fix case with received metric labels overriding generated.  


## v1.4.1
Released: 2023-06-09
- IMPROVEMENT: Update dependencies.


## v1.4.0
Released: 2023-05-06
- FEATURE: Reworked self-monitoring grafana dashboard for vmanomaly.
- IMPROVEMENT: Update python version and dependencies.


## v1.3.0
Released: 2023-03-21
- FEATURE: Parallelized queries. See `reader.workers` param to control parallelism. By default it's value is equal to number of queries (sends all the queries at once).
- IMPROVEMENT: Updated self-monitoring dashboard.
- IMPROVEMENT: Reverted back default bind address for /metrics server to 0.0.0.0, as vmanomaly is distributed in Docker images.
- IMPROVEMENT: Silenced Prophet INFO logs about yearly seasonality.

## v1.2.2
Released: 2023-03-19
- BUGFIX: Fix `for` metric label to pass QUERY_KEY.
- FEATURE: Added `timeout` config param to reader, writer, monitoring.push.
- BUGFIX: Don't hang if scheduler-model thread exits.
- FEATURE: Now reader, writer and monitoring.push will not halt the process if endpoint is inaccessible or times out, instead they will increment metrics `*_response_count{code=~"timeout|connection_error"}`.

## v1.2.1
Released: 2023-02-18
- BUGFIX: Fixed scheduler thread starting.
- BUGFIX: Fix rolling model fit+infer.
- BREAKING CHANGE: monitoring.pull server now binds by default on 127.0.0.1 instead of 0.0.0.0. Please specify explicitly in monitoring.pull.addr what IP address it should bind to for serving /metrics.

## v1.2.0
Released: 2023-02-04
- FEATURE: With arg `--watch` watches for config(s) changes and reloads the service automatically.
- IMPROVEMENT: Remove "provide_series" from HoltWinters model. Only Prophet model now has it, because it may produce a lot of series if "holidays" is on.
- IMPROVEMENT: if Prophet's "provide_series" is omitted, then all series are returned.
- DEPRECATION: Config monitoring.endpoint_url is deprecated in favor of monitoring.url.
- DEPRECATION: Remove 'enable' param from config monitoring.pull. Now /metrics server is started whenever monitoring.pull is present.
- IMPROVEMENT: include example configs into the docker image at /vmanomaly/config/*
- IMPROVEMENT: include self-monitoring grafana dashboard into the docker image under /vmanomaly/dashboard/vmanomaly_grafana_dashboard.json

## v1.1.0
Released: 2023-01-23
- IMPROVEMENT: update Python dependencies
- FEATURE: Add _multivariate_ IsolationForest model.

## v1.0.1
Released: 2023-01-06
- BUGFIX: prophet model incorrectly predicted two points in case of only one

## v1.0.0-beta
Released: 2022-12-08
- First public release is available
