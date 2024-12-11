---
title: Scheduler
weight: 3
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 3
aliases:
  - /anomaly-detection/components/scheduler.html
---
Scheduler defines how often to run and make inferences, as well as what timerange to use to train the model.
Is specified in `scheduler` section of a config for VictoriaMetrics Anomaly Detection.

> **Note: Starting from [v1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110) scheduler section in config supports multiple schedulers via aliasing. <br>Also, `vmanomaly` expects scheduler section to be named `schedulers`. Using old (flat) format with `scheduler` key is deprecated and will be removed in future versions.**

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

Old-style configs (< [1.11.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1110))

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

-  `"scheduler.periodic.PeriodicScheduler"`: periodically runs the models on new data. Useful for consecutive re-trainings to counter [data drift](https://www.datacamp.com/tutorial/understanding-data-drift-model-drift) and model degradation over time.
-  `"scheduler.oneoff.OneoffScheduler"`: runs the process once and exits. Useful for testing.
-  `"scheduler.backtesting.BacktestingScheduler"`: imitates consecutive backtesting runs of OneoffScheduler. Runs the process once and exits. Use to get more granular control over testing on historical data.

> **Note**: starting from [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130), class aliases are supported, so `"scheduler.periodic.PeriodicScheduler"` can be substituted to `"periodic"`, `"scheduler.oneoff.OneoffScheduler"` - to `"oneoff"`, `"scheduler.backtesting.BacktestingScheduler"` - to `"backtesting"`

**Depending on selected class, different parameters should be used**

## Periodic scheduler 

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
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`fit_window`
            </td>
            <td>str</td>
            <td>

`14d`
            </td>
            <td>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>

`infer_every`
            </td>
            <td>str</td>
            <td>

`1m`
            </td>
            <td>How often a model produce and write its anomaly scores on new datapoints. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>

`fit_every`
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

`start_from`
            </td>
            <td>str, Optional</td>
            <td>

`2024-11-26T01:00:00Z`, `01:00`
            </td>
            <td>

Available since [v1.18.5](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1185). Specifies when to initiate the first `fit_every` call. Accepts either an ISO 8601 datetime or a time in HH:MM format. If the specified time is in the past, the next suitable time is calculated based on the `fit_every` interval. For the HH:MM format, if the time is in the past, it will be scheduled for the same time on the following day, respecting the `tz` argument if provided. By default, the timezone defaults to `UTC`.
            </td>
        </tr>
        <tr>
            <td>

`tz`
            </td>
            <td>str, Optional</td>
            <td>

`America/New_York`
            </td>
            <td>

Available since [v1.18.5](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1185). Defines the local timezone for the `start_from` parameter, if specified. Defaults to `UTC` if no timezone is provided.
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
            <th>Format</th>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td>

`fit_start_iso`
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

`fit_start_s`
            </td>
            <td>float</td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

`fit_end_iso`
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

`fit_end_s`
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
            <th>Format</th>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td>

`infer_start_iso`
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

`infer_start_s`
            </td>
            <td>float</td>
            <td>1649635200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

`infer_end_iso`
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

`infer_end_s`
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
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

`n_jobs`
            </td>
            <td>int</td>
            <td>

`1`
            </td>
            <td>

Allows *proportionally faster (yet more resource-intensive)* evaluations of a config on historical data. Default value is 1, that implies *sequential* execution. Introduced in [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130)
            </td>
        </tr>
    </tbody>
</table>

### Defining overall timeframe

This timeframe will be used for slicing on intervals `(fit_window, infer_window == fit_every)`, starting from the *latest available* time point, which is `to_*` and going back, until no full `fit_window + infer_window` interval exists within the provided timeframe.
<table class="params">
    <thead>
        <tr>
            <th>Format</th>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td>

`from_iso`
            </td>
            <td>str</td>
            <td>

`"2022-04-01T00:00:00Z", "2022-04-01T00:00:00+01:00", "2022-04-01T00:00:00+0100", "2022-04-01T00:00:00+01"`
            </td>
            <td rowspan=2>Start datetime to use for backtesting.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td>

`from_s`
            </td>
            <td>float</td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td>

`to_iso`
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

`to_s`
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
            <th>Format</th>
            <th>Parameter</th>
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td rowspan=2>

`fit_window`
            </td>
            <td rowspan=2>str</td>
            <td>

`"PT1M", "P1H"`
            </td>
            <td rowspan=2>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td>

`"1m", "1h"`
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
            <th>Type</th>
            <th>Example</th>
            <th>Description</th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>ISO 8601</td>
            <td rowspan=2>

`fit_every`
            </td>
            <td rowspan=2>str</td>
            <td>

`"PT1M", "P1H"`
            </td>
            <td rowspan=2>What time range to use previously trained model to infer on new data until next retrain happens.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td>

`"1m", "1h"`
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
