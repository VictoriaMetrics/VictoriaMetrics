---
title: Scheduler
weight: 3
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 3
tags:
  - metrics
  - enterprise
aliases:
  - /anomaly-detection/components/scheduler.html
---

Scheduler defines how often to run and make inferences, as well as what timerange to use to train the model.
Is specified in `scheduler` section of a config for VictoriaMetrics Anomaly Detection.

> Scheduler section in config supports multiple schedulers via **aliasing** {{% available_from "v1.11.0" anomaly %}}. <br>Also, `vmanomaly` expects scheduler section to be named `schedulers`. Using old (flat) format with `scheduler` key is deprecated and will be removed in future versions.

```yaml
schedulers:
  scheduler_periodic_1m:
    # class: "periodic" # or class: "scheduler.periodic.PeriodicScheduler" until v1.13.0 with class alias support)
    infer_every: "1m"
    fit_every: "2m"
    fit_window: "3h"
  scheduler_periodic_5m:
    # class: "periodic" # or class: "scheduler.periodic.PeriodicScheduler" until v1.13.0 with class alias support)
    infer_every: "5m"
    fit_every: "10m"
    fit_window: "3h"
...
```  

Old-style configs {{% deprecated_from "v1.11.0" anomaly %}}

```yaml
scheduler:
  # class: "periodic" # or class: "scheduler.periodic.PeriodicScheduler" until v1.13.0 with class alias support)
  infer_every: "1m"
  fit_every: "2m"
  fit_window: "3h"
...
```

will be **implicitly** converted to

```yaml
schedulers:
  default_scheduler:  # default scheduler alias added, for backward compatibility
    class: "scheduler.periodic.PeriodicScheduler"
    infer_every: "1m"
    fit_every: "2m"
    fit_window: "3h"
...
```

## Parameters

`class`: str, default=`"scheduler.periodic.PeriodicScheduler"`,
options={`"scheduler.periodic.PeriodicScheduler"`, `"scheduler.oneoff.OneoffScheduler"`, `"scheduler.backtesting.BacktestingScheduler"`}

-  `"scheduler.periodic.PeriodicScheduler"`: Used in production. Periodically runs the models on new data to generate [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score). Also does consecutive re-trainings of attached [models](https://docs.victoriametrics.com/anomaly-detection/components/models/) to counter [data drift](https://www.datacamp.com/tutorial/understanding-data-drift-model-drift) and model degradation over time. 
-  `"scheduler.oneoff.OneoffScheduler"`: Runs the job once and exits. Useful for testing or for one-off backfilling on historical data.
-  `"scheduler.backtesting.BacktestingScheduler"`: Imitates running PeriodicScheduler, but runs only once and exits. Used for [backtesting](https://en.wikipedia.org/wiki/Backtesting) or for consecutive backfilling on historical data. One may evaluate the model performance on past data which already contained labeled incidents, to see how well the model would have performed in the past. See [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/#how-to-backtest-particular-configuration-on-historical-data) for the example and [BacktestingScheduler](#backtesting-scheduler) section below for the configuration details.

> **Class aliases** are supported{{% available_from "v1.13.0" anomaly %}}, so `"scheduler.periodic.PeriodicScheduler"` can be substituted to `"periodic"`, `"scheduler.oneoff.OneoffScheduler"` - to `"oneoff"`, `"scheduler.backtesting.BacktestingScheduler"` - to `"backtesting"`

**Depending on selected class, different parameters should be used**

## Periodic scheduler 

> If `start_from` [parameter](#parameters-1) is used, it's suggested to also set `restore_state: true` in the [Settings section](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration) of a config, so that the scheduler can restore its state from the previous run **if terminated or restarted in between scheduled runs** and continue producing anomaly scores without interruptions, otherwise the service will be idle until future `start_from` time is reached. E.g. if `start_from` is set to `20:00` and the service is started and then terminated and restarted at `20:30`, it will not produce any anomaly scores until the next day's `20:00` is reached (+23:30 of being idle), which introduces inconvenience for the users.

### Parameters

For periodic scheduler parameters are defined as differences in times, expressed in difference units, e.g. days, hours, minutes, seconds.

Examples: `"50s"`, `"4m"`, `"3h"`, `"2d"`, `"1w"`. 

<table class="params">
    <thead>
        <tr>
            <th></th>
            <th>Time granularity</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>s</td>
            <td>seconds</td>
        </tr>
        <tr>
            <td>m</td>
            <td>minutes</td>
        </tr>
        <tr>
            <td>h</td>
            <td>hours</td>
        </tr>
        <tr>
            <td>d</td>
            <td>days</td>
        </tr>
        <tr>
            <td>w</td>
            <td>weeks</td>
        </tr>
    </tbody>
</table>

<table class="params">
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`fit_window`</span>
            </td>
            <td>str</td>
            <td>

`14d`
            </td>
            <td>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`infer_every`</span>
            </td>
            <td>str</td>
            <td>

`1m`
            </td>
            <td>How often a model produce and write its anomaly scores on new datapoints. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`fit_every`</span>
            </td>
            <td>str, Optional</td>
            <td>

`1h`
            </td>
            <td>

How often to completely retrain the models. If not set, value of `infer_every` is used and retrain happens on every inference run.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`start_from`{{% available_from "v1.18.5" anomaly %}}</span>
            </td>
            <td>str, <span style="white-space: nowrap;">Optional</span></td>
            <td>

<span style="white-space: nowrap;">`2024-11-26T01:00:00Z`</span>, `01:00`
            </td>
            <td>

Specifies when to initiate the first `fit_every` call. Accepts either an ISO 8601 datetime or a time in HH:MM format. If the specified time is in the past, the next suitable time is calculated based on the `fit_every` interval. For the HH:MM format, if the time is in the past, it will be scheduled for the same time on the following day, respecting the `tz` argument if provided. By default, the timezone defaults to `UTC`.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tz`{{% available_from "v1.18.5" anomaly %}}</span>
            </td>
            <td>str, <span style="white-space: nowrap;">Optional</span></td>
            <td>

`America/New_York`
            </td>
            <td>

Defines the local timezone for the `start_from` parameter, if specified. Defaults to `UTC` if no timezone is provided.
            </td>
        </tr>
    </tbody>
</table>

### Periodic scheduler config example

```yaml
schedulers:
  periodic_scheduler_alias:
    class: "periodic"
    # (or class: "scheduler.periodic.PeriodicScheduler" for versions before v1.13.0, without class alias support)
    fit_window: "14d" 
    infer_every: "1m" 
    fit_every: "1h"
    start_from: "20:00"  # If launched before 20:00 (local Kyiv time), the first run starts today at 20:00. Otherwise, it starts tomorrow at 20:00.
    tz: "Europe/Kyiv"  # Defaults to 'UTC' if not specified.
```

This configuration specifies that `vmanomaly` will calculate a 14-day time window from the time of `fit_every` call to train the model. Starting at 20:00 Kyiv local time today (or tomorrow if launched after 20:00), the model will be retrained every hour using the most recent 14-day window, which always includes an additional hour of new data. The time window remains strictly 14 days and does not extend with subsequent retrains. Additionally, `vmanomaly` will perform model inference every minute, processing newly added data points using the most recent model.

## Oneoff scheduler

> As of latest version, the Oneoff scheduler can't be explicitly used with a combination of [stateful service](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration). It is designed to run once and exit, so it does not maintain state across runs. A warning will be raised in logs and internal state for such scheduler will not be saved and restored upon restart. If you need to run the scheduler periodically and/or maintain state, consider using the [Periodic scheduler](#periodic-scheduler) instead.

### Parameters
For Oneoff scheduler timeframes can be defined in Unix time in seconds or ISO 8601 string format. 
ISO format supported time zone offset formats are:
* Z (UTC)
* ±HH:MM
* ±HHMM
* ±HH

If a time zone is omitted, a timezone-naive datetime is used.

### Defining fitting timeframe
<table class="params">
    <thead>
        <tr>
            <th><span style="white-space: nowrap;">Format</span></th>
            <th>Parameter</th>
            <th><span style="white-space: nowrap;">Type</span></th>
            <th>Example</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td>

<span style="white-space: nowrap;">`fit_start_iso`</span>
            </td>
            <td>str</td>
            <td>

`"2022-04-01T00:00:00Z", "2022-04-01T00:00:00+01:00", "2022-04-01T00:00:00+0100", "2022-04-01T00:00:00+01"`
            </td>
            <td rowspan=2>Start datetime to use for training a model. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`fit_start_s`</span>
            </td>
            <td>

<span style="white-space: nowrap;">float</span>
            </td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

<span style="white-space: nowrap;">`fit_end_iso`</span>
            </td>
            <td>str</td>
            <td>

`"2022-04-10T00:00:00Z", "2022-04-10T00:00:00+01:00", "2022-04-10T00:00:00+0100", "2022-04-10T00:00:00+01"`
            </td>
            <td rowspan=2>End datetime to use for training a model. Must be greater than 

`fit_start_*`
. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`fit_end_s`</span>
            </td>
            <td>float</td>
            <td>1649548800</td>
        </tr>
    </tbody>
</table>

### Defining inference timeframe
<table class="params">
    <thead>
        <tr>
            <th><span style="white-space: nowrap;">Format</span></th>
            <th>Parameter</th>
            <th><span style="white-space: nowrap;">Type</span></th>
            <th>Example</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td>

<span style="white-space: nowrap;">`infer_start_iso`</span>
            </td>
            <td>str</td>
            <td>

`"2022-04-11T00:00:00Z", "2022-04-11T00:00:00+01:00", "2022-04-11T00:00:00+0100", "2022-04-11T00:00:00+01"`
            </td>
            <td rowspan=2>Start datetime to use for a model inference. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`infer_start_s`</span>
            </td>
            <td>

<span style="white-space: nowrap;">float</span>
            </td>
            <td>1649635200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

<span style="white-space: nowrap;">`infer_end_iso`</span>
            </td>
            <td>str</td>
            <td>

`"2022-04-14T00:00:00Z", "2022-04-14T00:00:00+01:00", "2022-04-14T00:00:00+0100", "2022-04-14T00:00:00+01"`
            </td>
            <td rowspan=2>End datetime to use for a model inference. Must be greater than 

`infer_start_*`
. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`infer_end_s`</span>
            </td>
            <td>float</td>
            <td>1649894400</td>
        </tr>
    </tbody>
</table>

### ISO format scheduler config example
```yaml
schedulers:
  oneoff_scheduler_alias:
    class: "oneoff"
    # (or class: "scheduler.oneoff.OneoffScheduler" until v1.13.0 with class alias support)
    fit_start_iso: "2022-04-01T00:00:00Z"
    fit_end_iso: "2022-04-10T00:00:00Z"
    infer_start_iso: "2022-04-11T00:00:00Z"
    infer_end_iso: "2022-04-14T00:00:00Z"
```


### UNIX time format scheduler config example               
```yaml
schedulers:
  oneoff_scheduler_alias:
    class: "oneoff"
    # (or class: "scheduler.oneoff.OneoffScheduler" until v1.13.0 with class alias support)
    fit_start_s: 1648771200
    fit_end_s: 1649548800
    infer_start_s: 1649635200
    infer_end_s: 1649894400
```

## Backtesting scheduler

> {{% available_from "v1.26.0" anomaly %}} `BacktestingScheduler` in [inference-only](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#inference-only-mode) mode is used in UI for backtesting configurations on historical data to verify that it works as expected before it goes live. See [vmanomaly UI](https://docs.victoriametrics.com/anomaly-detection/ui/) on how to access and use the UI.

> As of latest version, the Backtesting scheduler can't be explicitly used with a combination of [state restoration](https://docs.victoriametrics.com/anomaly-detection/components/settings/#state-restoration). It is designed to run once and exit, so it does not maintain state across runs. A warning will be raised in logs and internal state for such scheduler will not be saved and restored upon restart. If you need to run the scheduler periodically and/or maintain state, consider using the [Periodic scheduler](#periodic-scheduler) instead.

> A new, more intuitive backtesting mode is available {{% available_from "v1.22.1" anomaly %}}. In **Inference only** mode, the window you specify via `[from, to]` (or `[from_iso, to_iso]`) is used *solely for inference*, and the corresponding training (“fit”) windows are determined automatically. To enable this behavior, set:
> ```yaml
> inference_only: true
> ```
>
> in your scheduler configuration. (The default is `false` for backward-compatibility.) For full details, see [Inference only mode](#inference-only-mode).


### Parameters
As for [Oneoff scheduler](#oneoff-scheduler), timeframes can be defined in Unix time in seconds or ISO 8601 string format. 
ISO format supported time zone offset formats are:
* Z (UTC)
* ±HH:MM
* ±HHMM
* ±HH

If a time zone is omitted, a timezone-naive datetime is used.

### Parallelization
<table class="params">
    <thead>
        <tr>
            <th>

<span style="white-space: nowrap;">Parameter</span>
</th>
            <th>

<span style="white-space: nowrap;">Type</span>
            </th>
            <th>

<span style="white-space: nowrap;">Example</span>
</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`n_jobs`</span>
            </td>
            <td><span style="white-space: nowrap;">int</span></td>
            <td>

`1`
            </td>
            <td>

Allows *proportionally faster (yet more resource-intensive)* evaluations of a config on historical data. Default value is 1, that implies *sequential* execution. Introduced in [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130)
            </td>
        </tr>
    </tbody>
</table>

### Inference only mode

In **Inference only** mode {{% available_from "v1.22.1" anomaly %}}, the scheduler splits your overall time window into non-overlapping inference segments and automatically derives the preceding training segments:

1. **Inference Window**  
   - Defined by your `from`/`to` (or `from_iso`/`to_iso`) parameters.
   - Each inference segment spans the configured `fit_every` duration.
2. **Training Window**  
   - Automatically set to the configured `fit_window` immediately preceding each inference segment.
   - Ensures each model is trained on the most recent `fit_window` of data before inferring, see [example](#example) section for the details

#### Configuration Parameters
- `inference_only: true`: Enables of such inference-only behavior.
- `from`, `to` (or `from_iso`, `to_iso`): Overall inference-only timeframe.
- `fit_window`: Duration of historical data used for each training run (e.g. `P7D`, `PT1H`).
- `fit_every`: Interval between consecutive training/inference cycles.
- `n_jobs`: Number of parallel jobs for backtesting (default: `1`).

#### Example

The config

```yaml
# other config sections ...
schedulers:
  backtesting_inference_only:       # scheduler alias
    class: "backtesting"
    fit_window: "P7D"               # train on the 7-day window preceding each inference
    fit_every: "PT12H"              # inference interval of 12 hours
    inference_only: true            # use [from, to] to construct inference windows only
    from_iso: "2025-05-08T03:00:00Z"
    to_iso:   "2025-05-09T00:00:00Z"
    n_jobs: 2                       # number of parallel jobs
```

will result in 2 intervals:

- Complete inference interval (12h): `2025-05-08T12:00:00Z` - `2025-05-09T00:00:00Z`
<br>Training window (7d): `2025-05-01T12:00:00Z` - `2025-05-08T12:00:00Z`

- Partial inference interval (9h): `2025-05-08T03:00:00Z` - `2025-05-08T12:00:00Z` 
<br>(start is "clipped" by `from_iso` so it's less than `fit_every`)
<br>Training window (7d): `2025-05-01T03:00:00Z` - `2025-05-08T03:00:00Z`

Where models, fit on training window, will be used to calculate [anomaly scores](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) on respective inference windows.

### Defining overall timeframe

> This legacy mode is retained for backward compatibility and is less straightforward. Use it only if you cannot upgrade `vmanomaly` to [v1.22.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1221) or later.

This timeframe will be used for slicing on intervals `(fit_window, infer_window == fit_every)`, starting from the *latest available* time point, which is `to_*` and going back, until no full `fit_window + infer_window` interval exists within the provided timeframe.
<table class="params">
    <thead>
        <tr>
            <th>

<span style="white-space: nowrap;">Format</span>
            </th>
            <th>

<span style="white-space: nowrap;">Parameter</span>
            </th>
            <th>

<span style="white-space: nowrap;">Type</span>
            </th>
            <th>

<span style="white-space: nowrap;">Example</span>
            </th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td><span style="white-space: nowrap;">ISO 8601</span></td>
            <td>

<span style="white-space: nowrap;">`from_iso`</span>
            </td>
            <td><span style="white-space: nowrap;">str</span></td>
            <td>

`"2022-04-01T00:00:00Z", "2022-04-01T00:00:00+01:00", "2022-04-01T00:00:00+0100", "2022-04-01T00:00:00+01"`
            </td>
            <td rowspan=2>Start datetime to use for backtesting.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`from_s`</span>
            </td>
            <td>float</td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

<span style="white-space: nowrap;">`to_iso`</span>
            </td>
            <td>str</td>
            <td>

`"2022-04-10T00:00:00Z", "2022-04-10T00:00:00+01:00", "2022-04-10T00:00:00+0100", "2022-04-10T00:00:00+01"`
            </td>
            <td rowspan=2>End datetime to use for backtesting. Must be greater than 

`from_start_*`
            </td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

<span style="white-space: nowrap;">`to_s`</span>
            </td>
            <td>float</td>
            <td>1649548800</td>
        </tr>
    </tbody>
</table>

### Defining training timeframe
The same *explicit* logic as in [Periodic scheduler](#periodic-scheduler)
<table class="params">
    <thead>
        <tr>
            <th><span style="white-space: nowrap;">Format</span></th>
            <th>Parameter</th>
            <th><span style="white-space: nowrap;">Type</span></th>
            <th><span style="white-space: nowrap;">Example</span></th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td rowspan=2>

<span style="white-space: nowrap;">`fit_window`</span>
            </td>
            <td rowspan=2>str</td>
            <td>

<span style="white-space: nowrap;">`"PT1M"`</span>, `"P1H"`
            </td>
            <td rowspan=2>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td>

<span style="white-space: nowrap;">`"1m"`</span>, `"1h"`
            </td>
        </tr>
    </tbody>
</table>

### Defining inference timeframe
In `BacktestingScheduler`, the inference window is *implicitly* defined as a period between 2 consecutive model `fit_every` runs. The *latest* inference window starts from `to_s` - `fit_every` and ends on the *latest available* time point, which is `to_s`. The previous periods for fit/infer are defined the same way, by shifting `fit_every` seconds backwards until we get the last full fit period of `fit_window` size, which start is >= `from_s`.
<table class="params">
    <thead>
        <tr>
            <th>Format</th>
            <th>Parameter</th>
            <th><span style="white-space: nowrap;">Type</span></th>
            <th><span style="white-space: nowrap;">Example</span></th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td rowspan=2>

<span style="white-space: nowrap;">`fit_every`</span>
            </td>
            <td rowspan=2>str</td>
            <td>

<span style="white-space: nowrap;">`"PT1M"`</span>, `"P1H"`
            </td>
            <td rowspan=2>What time range to use previously trained model to infer on new data until next retrain happens.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td>

<span style="white-space: nowrap;">`"1m"`</span>, `"1h"`
            </td>
        </tr>
    </tbody>
</table>

### ISO format scheduler config example
```yaml
schedulers:
  backtesting_scheduler_alias:
    class: "backtesting"
    # (or class: "scheduler.backtesting.BacktestingScheduler" until v1.13.0 with class alias support)
    from_iso: '2021-01-01T00:00:00Z'
    to_iso: '2021-01-14T00:00:00Z'
    fit_window: 'P14D'
    fit_every: 'PT1H'
    n_jobs: 1  # default = 1 (sequential), set it up to # of CPUs for parallel execution
```

### UNIX time format scheduler config example                 
```yaml
schedulers:
  backtesting_scheduler_alias:
    class: "backtesting"
    # (or class: "scheduler.backtesting.BacktestingScheduler" until v1.13.0 with class alias support)
    from_s: 167253120
    to_s: 167443200
    fit_window: '14d'
    fit_every: '1h'
    n_jobs: 1  # default = 1 (sequential), set it up to # of CPUs for parallel execution
```
