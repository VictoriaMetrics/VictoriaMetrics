---
weight: 5
title: CHANGELOG
menu:
  docs:
    identifier: "vmanomaly-changelog"
    parent: "anomaly-detection"
    weight: 5
aliases:
- /anomaly-detection/CHANGELOG.html
---
Please find the changelog for VictoriaMetrics Anomaly Detection below.

## v1.16.3
Released: 2024-10-08
- IMPROVEMENT: Added `tls_cert_file` and `tls_key_file` arguments to support mTLS (mutual TLS) in `vmanomaly` components. This enhancement applies to the following components: [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer), and [Monitoring/Push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters). You can also use these arguments in conjunction with `verify_tls` when it is set as a path to a custom CA certificate file.

## v1.16.2
Released: 2024-10-06
- FEATURE: Added support for `multitenant` value in `tenant_id` arg to enable querying across multiple tenants in [VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/) (option available from [v1.104.0](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy-via-labels)):
  - Applied when reading input data from `vmselect` via the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader#vm-reader).
  - Applied when writing generated results through `vminsert` via the [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer#vm-writer).
  - For more details, refer to the `tenant_id` arg description in the documentation of the mentioned components.

- FIX: Resolved an issue with handling an empty `preset` value (e.g., `preset: ""`) that was preventing the [default helm chart](https://github.com/VictoriaMetrics/helm-charts/blob/7f5a2c00b14c2c088d7d8d8bcee7a440a5ff11c6/charts/victoria-metrics-anomaly/values.yaml#L139) from being deployed.

## v1.16.1
Released: 2024-10-02
- FIX: This patch release prevents the service from crashing by rolling back the version of a third-party dependency. Affected releases: [v1.16.0](#v1160).

## v1.16.0
Released: 2024-10-01

> **Note**: A bug was discovered in this release that causes the service to crash. Please use the patch [v1.16.1](#v1161) to resolve this issue.

- FEATURE: Introduced data dumps to a host filesystem for [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader#vm-reader).  Resource-intensive setups (multiple queries returning many metrics, bigger `fit_window` arg) will have RAM consumption reduced during fit calls.
- IMPROVEMENT: Added a `groupby` argument for logical grouping in [multivariate models](https://docs.victoriametrics.com/anomaly-detection/components/models#multivariate-models). When specified, a separate multivariate model is trained for each unique combination of label values in the `groupby` columns. For example, to perform multivariate anomaly detection on metrics at the machine level without cross-entity interference, you can use `groupby: [host]` or `groupby: [instance]`, ensuring one model per entity being trained (e.g., per host). Please find more details [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#group-by).
- IMPROVEMENT: Improved performance of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader#vm-reader) on multicore instances for reading and data processing.
- IMPROVEMENT: Introduced new CLI argument aliases to enhance compatibility with [Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/README.md) (i.e. using secrets) and better align with [VictoriaMetrics flags](https://docs.victoriametrics.com/#list-of-command-line-flags):
  - `--licenseFile` as an alias for `--license-file`
  - `--license.forceOffline` as an alias for `--license-verify-offline`
  - `--loggerLevel` as an alias for `--log-level`
  - The previous argument format is retained for backward compatibility.

- FIX: The `provide_series` [common argument](https://docs.victoriametrics.com/anomaly-detection/components/models/#provide-series) now correctly filters the written time series in the [IsolationForestMultivariate](https://docs.victoriametrics.com/anomaly-detection/components/models/#isolation-forest-multivariate) model.

## v1.15.9
Released: 2024-08-27
- IMPROVEMENT: Added support for bearer token authentication in `push` mode within the [self-monitoring configuration section](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters).

## v1.15.8
Released: 2024-08-27
- FIX: Made minor adjustments to how the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) handle bearer tokens across different modes.

## v1.15.7
Released: 2024-08-27
- FIX: Made minor adjustments to how the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) handle bearer tokens across different modes.

## v1.15.6
Released: 2024-08-26
- IMPROVEMENT: Introduced the `bearer_token_file` argument to the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer) components to enhance secret management.


## v1.15.5
Released: 2024-08-19
- FIX: following [v1.15.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1152) online model enhancement, now `data_range` parameter is correctly initialized for online models, created (for new time series returned by particular query) during `infer` calls. 

## v1.15.4
Released: 2024-08-15
- IMPROVEMENT: better config handling of [writer](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) and [monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/) sections if using `vmanomaly` with [helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).

## v1.15.3
Released: 2024-08-14
- IMPROVEMENT: better config handling of `reader` [section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) if using `vmanomaly` with [helm charts](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-anomaly).

## v1.15.2
Released: 2024-08-13
- IMPROVEMENT: Enhanced [online models](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-models) (e.g., [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile)) to automatically create model instances for unseen time series during `infer` calls, eliminating the need to wait for the next `fit` call. This ensures no inferences are skipped **when using online models**.
- FIX: Corrected an issue with the [`OnlineMADModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-mad) to ensure proper functionality when used in combination with [on-disk model dump mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).
- FIX: Addressed numerical instability in the [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) when `use_transform` is set to `True`.
- FIX: Resolved a logging issue that could cause a `RuntimeError: reentrant call inside <_io.BufferedWriter name='<stderr>'>` when a termination event was received.

## v1.15.1
Released: 2024-08-10
- FEATURE: Introduced backward-compatible `data_range` [query-specific parameter](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters) to the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader). It enables the definition of **valid** data ranges for input per individual query in `queries`, resulting in:
  - **High anomaly scores** (>1) when the *data falls outside the expected range*, indicating a data constraint violation.
  - **Lowest anomaly scores** (=0) when the *model's predictions (`yhat`) fall outside the expected range*, signaling uncertain predictions.
  - For more details, please refer to the [documentation](https://docs.victoriametrics.com/anomaly-detection/components/reader/?highlight=data_range#per-query-parameters).

- IMPROVEMENT: Added `latency_offset` argument to the [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to override the default `-search.latencyOffset` [flag of VictoriaMetrics](https://docs.victoriametrics.com/?highlight=search.latencyOffset#list-of-command-line-flags) (30s). The default value is set to 1ms, which should help in cases where `sampling_frequency` is low (10-60s) and `sampling_frequency` equals `infer_every` in the [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/?highlight=infer_every#periodic-scheduler). This prevents users from receiving `service - WARNING - [Scheduler [scheduler_alias]] No data available for inference.` warnings in logs and allows for consecutive `infer` calls without gaps. To restore the backward compatible behavior, set it equal to your `-search.latencyOffset` value in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) config section.

- FIX: Ensure the `use_transform` argument of the [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile) functions as intended.
- FIX: Add a docstring for `query_from_last_seen_timestamp` arg of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader).


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
- FIX: Patch a bug introduced in [v1.14.1](#v1141), causing `vmanomaly` to crash in `preset` [mode](https://docs.victoriametrics.com/anomaly-detection/presets/).

## v1.14.1
Released: 2024-07-26
- FEATURE: Allow to process larger data chunks in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader#vm-reader) that exceed `-search.maxPointsPerTimeseries` [constraint in VictoriaMetrics](https://docs.victoriametrics.com/?highlight=search.maxPointsPerTimeseries#resource-usage-limits) by splitting the range and sending multiple requests. A warning is printed in logs, suggesting reducing the range or step, or increasing `search.maxPointsPerTimeseries` constraint in VictoriaMetrics, which is still a recommended option.
- FEATURE: Backward-compatible redesign of [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/reader?highlight=queries#vm-reader) arg of [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader#vm-reader). Old format of `{q_alias1: q_expr1, q_alias2: q_expr2, ...}` will be implicitly converted to a new one with a warning raised in logs. New format allows to specify per-query parameters, like `step` to reduce amount of data read from VictoriaMetrics TSDB and to allow config flexibility. Find out more in [Per-query parameters section of VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#per-query-parameters).

- IMPROVEMENT: Added multi-platform builds for `linux/amd64` and `linux/arm64` architectures.

## v1.13.3
Released: 2024-07-17
- FIX: now validation of `args` argument for [`HoltWinters`](https://docs.victoriametrics.com/anomaly-detection/components/models/#holt-winters) model works properly.

## v1.13.2
Released: 2024-07-15
- IMPROVEMENT: update `node-exporter` [preset](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter) to reduce [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive)
- FIX: add `verify_tls` arg for [`push`](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters) monitoring section. Also, `verify_tls` is now correctly used in [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer).
- FIX: now [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned) model wrapper works correctly in [on-disk model storage mode](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).
- FIX: now [rolling models](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-models), like [`RollingQuantile`](https://docs.victoriametrics.com/anomaly-detection/components/models/#rolling-quantile) are properly handled in [One-off scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#oneoff-scheduler), when wrapped in [`AutoTuned`](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned)

## v1.13.0
Released: 2024-06-11
- FEATURE: Introduced `preset` [mode to run vmanomaly service](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter) with minimal user input and on widely-known metrics, like those produced by [`node_exporter`](https://docs.victoriametrics.com/anomaly-detection/presets/#node-exporter).
- FEATURE: Introduced `min_dev_from_expected` [model common arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#minimal-deviation-from-expected), aimed at **reducing [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive)** in scenarios where deviations between the real value `y` and the expected value `yhat` are **relatively** high and may cause models to generate high [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score). However, these deviations are not significant enough in **absolute values** to be considered anomalies based on domain knowledge.
- FEATURE: Introduced `detection_direction` [model common arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#detection-direction), enabling domain-driven anomaly detection strategies. Configure models to identify anomalies occurring *above, below, or in both directions* relative to the expected values.
- FEATURE: add `n_jobs` arg to [`BacktestingScheduler`](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#backtesting-scheduler) to allow *proportionally faster (yet more resource-intensive)* evaluations of a config on historical data. Default value is 1, that implies *sequential* execution.
- FEATURE: allow anomaly detection models to be dumped to a host filesystem after `fit` stage (instead of in-memory). Resource-intensive setups (many models, many metrics, bigger [`fit_window` arg](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler-config-example)) and/or 3rd-party models that store fit data (like [ProphetModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or [HoltWinters](https://docs.victoriametrics.com/anomaly-detection/components/models/#holt-winters)) will have RAM consumption greatly reduced at a cost of slightly slower `infer` stage. Please find how to enable it [here](https://docs.victoriametrics.com/anomaly-detection/faq/#resource-consumption-of-vmanomaly)
- IMPROVEMENT: Reduced the resource used for each fitted [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) by up to 6 times. This includes both RAM for in-memory models and disk space for on-disk models storage. For more details, refer to [this discussion on Facebook's Prophet](https://github.com/facebook/prophet/issues/1159#issuecomment-537415637).
- IMPROVEMENT: now config [components](https://docs.victoriametrics.com/anomaly-detection/components/) class can be referenced by a short alias instead of a full class path - i.e. `model.zscore.ZscoreModel` becomes `zscore`, `reader.vm.VmReader` becomes `vm`, `scheduler.periodic.PeriodicScheduler` becomes `periodic`, etc.
- FIX: if using multi-scheduler setup (introduced in [v1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110)), prevent schedulers (and correspondent services) that are not attached to any model (so neither found in ['schedulers'  arg](https://docs.victoriametrics.com/anomaly-detection/components/models/#schedulers) nor left blank in `model` section) from being spawn, causing resource overhead and slight interference with existing ones.
- FIX: set random seed for [ProphetModel](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) to assure uncertainty estimates (like `yhat_lower`, `yhat_upper`) and dependant series (like `anomaly_score`), produced during `.infer()` calls are always deterministic given the same input. See [initial issue](https://github.com/facebook/prophet/issues/1124) for the details.
- FIX: prevent *orphan* queries (that are not attached to any model or scheduler) found in `queries` arg of [Reader config section](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader) to be fetched from VictoriaMetrics TSDB, avoiding redundant data processing. A warning will be logged, if such queries exist in a parsed config.

## v1.12.0
Released: 2024-03-31
- FEATURE: Introduction of `AutoTunedModel` model class to optimize any [built-in model](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models) on data during `fit` phase. Specify as little as `anomaly_percentage` param from `(0, 0.5)` interval and `tuned_model_class` (i.e. [`model.zscore.ZscoreModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#z-score)) to get it working with best settings that match your data. See details [here](https://docs.victoriametrics.com/anomaly-detection/components/models/#autotuned).
<!--
- FEATURE: Preset support enablement. From now users will be able to specify only a few parameters (like `datasource_url`) + a new (backward-compatible) `preset: preset_name` field in a config file and get a service run with **predefined queries, scheduling and models**. Also, now preset assets (guide, configs, dashboards) will be available at `:8490/presets` endpoint.
-->
- IMPROVEMENT: Better logging of model lifecycle (fit/infer stages).
- IMPROVEMENT: Introduce `provide_series` arg to all the [built-in models](https://docs.victoriametrics.com/anomaly-detection/components/models/#built-in-models) to define what output fields to generate for writing (i.e. `provide_series: ['anomaly_score']` means only scores are being produced)
- FIX: [Self-monitoring metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics) are now aggregated to `queries` aliases level (not to label sets of individual timeseries) and aligned with [reader, writer and model sections](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#metrics-generated-by-vmanomaly) description , so `/metrics` endpoint holds only necessary information for scraping.
- FIX: Self-monitoring metric `vmanomaly_models_active` now has additional labels `model_alias`, `scheduler_alias`, `preset` to align with model-centric [self-monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#models-behaviour-metrics).
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

> **Note**: These updates support more flexible setup and effective resource management in service, as now it's not longer needed to spawn several instances of `vmanomaly` to split queries/models context across.


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
> **Note**: The majority of users, who have been proactively specifying the `sampling_period` parameter in their configurations, will experience no disruption from this update. This transition formalizes a practice that was already prevalent and expected among our user base.


## v1.8.0
Released: 2024-01-15
- FEATURE: Added Univariate [MAD (median absolute deviation)](https://docs.victoriametrics.com/anomaly-detection/components/models/#mad-median-absolute-deviation) model support.
- IMPROVEMENT: Update Python to 3.12.1 and all the dependencies.
- IMPROVEMENT: Don't check /health endpoint, check the real /query_range or /import endpoints directly. Users kept getting problems with /health.
- DEPRECATION: "health_path" param is deprecated and doesn't do anything in config ([reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer), [monitoring.push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters)).


## v1.7.2
Released: 2023-12-21
- FIX: fit/infer calls are now skipped if we have insufficient *valid* data to run on.
- FIX: proper handling of `inf` and `NaN` in fit/infer calls.
- FEATURE: add counter of skipped model runs `vmanomaly_model_runs_skipped` to healthcheck metrics.
- FEATURE: add exponential retries wrapper to VmReader's `read_metrics()`.
- FEATURE: add `BacktestingScheduler` for consecutive retrospective fit/infer calls.
- FEATURE: add improved & numerically stable anomaly scores.
- IMPROVEMENT: add full config validation. The probability of getting errors in later stages (say, model fit) is greatly reduced now. All the config validation errors that needs to be fixed are now a part of logging.
  > **note**: this is an backward-incompatible change, as `model` config section now expects key-value args for internal model defined in nested `args`.
- IMPROVEMENT: add explicit support of `gzip`-ed responses from vmselect in VmReader.


## v1.6.0
Released: 2023-10-30
- IMPROVEMENT: 
  - now all the produced healthcheck metrics have `vmanomaly_` prefix for easier accessing.
  - updated docs for monitoring.
  > **note**: this is an backward-incompatible change, as metric names will be changed, resulting in new metrics creation, i.e. `model_datapoints_produced` will become `vmanomaly_model_datapoints_produced`
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
- FIX: Fix case with received metric labels overriding generated.  


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
- FIX: Fix `for` metric label to pass QUERY_KEY.
- FEATURE: Added `timeout` config param to reader, writer, monitoring.push.
- FIX: Don't hang if scheduler-model thread exits.
- FEATURE: Now reader, writer and monitoring.push will not halt the process if endpoint is inaccessible or times out, instead they will increment metrics `*_response_count{code=~"timeout|connection_error"}`.

## v1.2.1
Released: 2023-02-18
- FIX: Fixed scheduler thread starting.
- FIX: Fix rolling model fit+infer.
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
- FIX: prophet model incorrectly predicted two points in case of only one

## v1.0.0-beta
Released: 2022-12-08
- First public release is available
