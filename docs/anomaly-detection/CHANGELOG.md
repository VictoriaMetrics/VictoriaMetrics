---
sort: 3
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

# CHANGELOG

Please find the changelog for VictoriaMetrics Anomaly Detection below.

> **Important note: Users are strongly encouraged to upgrade to `vmanomaly` [v1.9.2](https://hub.docker.com/repository/docker/victoriametrics/vmanomaly/tags?page=1&ordering=name) or later versions for optimal performance and accuracy. <br><br> This recommendation is crucial for configurations with a low `infer_every` parameter [in your scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#parameters-1), and in scenarios where data exhibits significant high-order seasonality patterns (such as hourly or daily cycles). Previous versions from v1.5.1 to v1.8.0 were identified to contain a critical issue impacting model training, where models were inadvertently trained on limited data subsets, leading to suboptimal fits, affecting the accuracy of anomaly detection. <br><br> Upgrading to v1.9.2 addresses this issue, ensuring proper model training and enhanced reliability. For users utilizing Helm charts, it is recommended to upgrade to version [1.0.0](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/CHANGELOG.md#100) or newer.**

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
- FEATURE: Multi-model support. Now users can specify multiple [model specs](https://docs.victoriametrics.com/anomaly-detection/components/models/) in a single config (via aliasing), as well as to reference what [queries from VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/?highlight=queries#config-parameters) it should be run on.
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
- BUGFIX: The `query_from_last_seen_timestamp` internal logic in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader.html#vm-reader), first introduced in [v1.5.1](https://docs.victoriametrics.com/anomaly-detection/CHANGELOG.html#v151), now functions correctly. This fix ensures that the input data shape remains consistent for subsequent `fit`-based model calls in the service.
- BREAKING CHANGE: The `sampling_period` parameter is now mandatory in [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader.html#vm-reader). This change aims to clarify and standardize the frequency of input/output in `vmanomaly`, thereby reducing uncertainty and aligning with user expectations.
> **Note**: The majority of users, who have been proactively specifying the `sampling_period` parameter in their configurations, will experience no disruption from this update. This transition formalizes a practice that was already prevalent and expected among our user base.


## v1.8.0
Released: 2024-01-15
- FEATURE: Added Univariate [MAD (median absolute deviation)](/anomaly-detection/components/models.html#mad-median-absolute-deviation) model support.
- IMPROVEMENT: Update Python to 3.12.1 and all the dependencies.
- IMPROVEMENT: Don't check /health endpoint, check the real /query_range or /import endpoints directly. Users kept getting problems with /health.
- DEPRECATION: "health_path" param is deprecated and doesn't do anything in config ([reader](/anomaly-detection/components/reader.html#vm-reader), [writer](/anomaly-detection/components/writer.html#vm-writer), [monitoring.push](/anomaly-detection/components/monitoring.html#push-config-parameters)).


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
- DEPRECATION: Config monitoring.endpount_url is deprecated in favor of monitoring.url.
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