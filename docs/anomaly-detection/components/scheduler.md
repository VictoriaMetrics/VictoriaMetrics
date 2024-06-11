---
sort: 3
title: Scheduler
weight: 3
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 3
aliases:
  - /anomaly-detection/components/scheduler.html
---

# Scheduler

Scheduler defines how often to run and make inferences, as well as what timerange to use to train the model.
Is specified in `scheduler` section of a config for VictoriaMetrics Anomaly Detection.

> **Note: Starting from [v1.11.0](/anomaly-detection/changelog#v1110) scheduler section in config supports multiple schedulers via aliasing. <br>Also, `vmanomaly` expects scheduler section to be named `schedulers`. Using old (flat) format with `scheduler` key is deprecated and will be removed in future versions.**

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

Old-style configs (< [1.11.0](/anomaly-detection/changelog#v1110))

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

> **Note**: starting from [v.1.13.0](/anomaly-detection/CHANGELOG/#v1130), class aliases are supported, so `"scheduler.periodic.PeriodicScheduler"` can be substituted to `"periodic"`, `"scheduler.oneoff.OneoffScheduler"` - to `"oneoff"`, `"scheduler.backtesting.BacktestingScheduler"` - to `"backtesting"`

**Depending on selected class, different parameters should be used**

## Periodic scheduler 

### Parameters

For periodic scheduler parameters are defined as differences in times, expressed in difference units, e.g. days, hours, minutes, seconds.

Examples: `"50s"`, `"4m"`, `"3h"`, `"2d"`, `"1w"`. 

<table>
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

<table>
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
            <td><code>fit_window</code></td>
            <td>str</td>
            <td><code>"14d"</code></td>
            <td>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td><code>infer_every</code></td>
            <td>str</td>
            <td><code>"1m"</code></td>
            <td>How often a model will write its conclusions on newly added data. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td><code>fit_every</code></td>
            <td>str, Optional</td>
            <td><code>"1h"</code></td>
            <td>How often to completely retrain the models. If missing value of <code>infer_every</code> is used and retrain on every inference run.</td>
        </tr>
    </tbody>
</table>

### Periodic scheduler config example

```yaml
schedulers:
  periodic_scheduler_alias:
    class: "periodic"
    # (or class: "scheduler.periodic.PeriodicScheduler" until v1.13.0 with class alias support)
    fit_window: "14d" 
    infer_every: "1m" 
    fit_every: "1h" 
```

This part of the config means that `vmanomaly` will calculate the time window of the previous 14 days and use it to train a model. Every hour model will be retrained again on 14 days’ data, which will include + 1 hour of new data. The time window is strictly the same 14 days and doesn't extend for the next retrains. Every minute `vmanomaly` will produce model inferences for newly added data points by using the model that is kept in memory at that time.

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
<table>
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
            <td><code>fit_start_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-01T00:00:00Z", "2022-04-01T00:00:00+01:00", "2022-04-01T00:00:00+0100", "2022-04-01T00:00:00+01"</code></td>
            <td rowspan=2>Start datetime to use for training a model. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>fit_start_s</code></td>
            <td>float</td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td><code>fit_end_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-10T00:00:00Z", "2022-04-10T00:00:00+01:00", "2022-04-10T00:00:00+0100", "2022-04-10T00:00:00+01"</code></td>
            <td rowspan=2>End datetime to use for training a model. Must be greater than <code>fit_start_*</code>. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>fit_end_s</code></td>
            <td>float</td>
            <td>1649548800</td>
        </tr>
    </tbody>
</table>

### Defining inference timeframe
<table>
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
            <td><code>infer_start_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-11T00:00:00Z", "2022-04-11T00:00:00+01:00", "2022-04-11T00:00:00+0100", "2022-04-11T00:00:00+01"</code></td>
            <td rowspan=2>Start datetime to use for a model inference. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>infer_start_s</code></td>
            <td>float</td>
            <td>1649635200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td><code>infer_end_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-14T00:00:00Z", "2022-04-14T00:00:00+01:00", "2022-04-14T00:00:00+0100", "2022-04-14T00:00:00+01"</code></td>
            <td rowspan=2>End datetime to use for a model inference. Must be greater than <code>infer_start_*</code>. ISO string or UNIX time in seconds.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>infer_end_s</code></td>
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
<table>
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
            <td><code>n_jobs</code></td>
            <td>int</td>
            <td><code>1</code></td>
            <td>Allows <i>proportionally faster (yet more resource-intensive)</i> evaluations of a config on historical data. Default value is 1, that implies <i>sequential</i> execution. Introduced in <a href="https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130">v1.13.0</a></td>
        </tr>
    </tbody>
</table>

### Defining overall timeframe

This timeframe will be used for slicing on intervals `(fit_window, infer_window == fit_every)`, starting from the *latest available* time point, which is `to_*` and going back, until no full `fit_window + infer_window` interval exists within the provided timeframe.
<table>
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
            <td><code>from_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-01T00:00:00Z", "2022-04-01T00:00:00+01:00", "2022-04-01T00:00:00+0100", "2022-04-01T00:00:00+01"</code></td>
            <td rowspan=2>Start datetime to use for backtesting.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>from_s</code></td>
            <td>float</td>
            <td>1648771200</td>
        </tr>
        <tr>
            <td>ISO 8601</td>
            <td><code>to_iso</code></td>
            <td>str</td>
            <td><code>"2022-04-10T00:00:00Z", "2022-04-10T00:00:00+01:00", "2022-04-10T00:00:00+0100", "2022-04-10T00:00:00+01"</code></td>
            <td rowspan=2>End datetime to use for backtesting. Must be greater than <code>from_start_*</code>.</td>
        </tr>
        <tr>
            <td>UNIX time</td>
            <td><code>to_s</code></td>
            <td>float</td>
            <td>1649548800</td>
        </tr>
    </tbody>
</table>

### Defining training timeframe
The same *explicit* logic as in [Periodic scheduler](#periodic-scheduler)
<table>
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
            <td rowspan=2><code>fit_window</code></td>
            <td rowspan=2>str</td>
            <td><code>"PT1M", "P1H"</code></td>
            <td rowspan=2>What time range to use for training the models. Must be at least 1 second.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td><code>"1m", "1h"</code></td>
        </tr>
    </tbody>
</table>

### Defining inference timeframe
In `BacktestingScheduler`, the inference window is *implicitly* defined as a period between 2 consecutive model `fit_every` runs. The *latest* inference window starts from `to_s` - `fit_every` and ends on the *latest available* time point, which is `to_s`. The previous periods for fit/infer are defined the same way, by shifting `fit_every` seconds backwards until we get the last full fit period of `fit_window` size, which start is >= `from_s`.
<table>
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
            <td rowspan=2><code>fit_every</code></td>
            <td rowspan=2>str</td>
            <td><code>"PT1M", "P1H"</code></td>
            <td rowspan=2>What time range to use previously trained model to infer on new data until next retrain happens.</td>
        </tr>
        <tr>
            <td>Prometheus-compatible</td>
            <td><code>"1m", "1h"</code></td>
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