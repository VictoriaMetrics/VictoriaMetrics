---
title: Models
weight: 1
sort: 1
menu:
  docs:
    identifier: "vmanomaly-models"
    parent: "vmanomaly-components"
    weight: 1
    # sort: 1
aliases:
  - /anomaly-detection/components/models.html
  - /anomaly-detection/components/models/custom_model.html
  - /anomaly-detection/components/models/models.html
---

# Models

This section describes `Models` component of VictoriaMetrics Anomaly Detection (or simply [`vmanomaly`](/anomaly-detection/overview/)) and the guide of how to define a respective section of a config to launch the service.
- `vmanomaly` includes various [built-in models](#built-in-models).
- you can also integrate your custom model - see [custom model](#custom-model-guide).


> **Note: Starting from [v1.10.0](/anomaly-detection/changelog#v1100) model section in config supports multiple models via aliasing. <br>Also, `vmanomaly` expects model section to be named `models`. Using old (flat) format with `model` key is deprecated and will be removed in future versions. Having `model` and `models` sections simultaneously in a config will result in only `models` being used:**

```yaml
models:
  model_univariate_1:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 2.5
    queries: ['query_alias2']  # referencing queries defined in `reader` section
  model_multivariate_1:
    class: 'isolation_forest_multivariate'  # or model.isolation_forest.IsolationForestMultivariateModel until v1.13.0
    contamination: 'auto'
    args:
      n_estimators: 100
      # i.e. to assure reproducibility of produced results each time model is fit on the same input
      random_state: 42
    # if there is no explicit `queries` arg, then the model will be run on ALL queries found in reader section
# ...
```  

Old-style configs (< [1.10.0](/anomaly-detection/changelog#v1100))

```yaml
model:
    class: "zscore"  # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3.0
    # no explicit `queries` arg is provided
# ...
```

will be **implicitly** converted to

```yaml
models:
  default_model:  # default model alias, backward compatibility
    class: "model.zscore.ZscoreModel"
    z_threshold: 3.0
    # queries arg is created and propagated with all query aliases found in `queries` arg of `reader` section
    queries: ['q1', 'q2', 'q3']  # i.e., if your `queries` in `reader` section has exactly q1, q2, q3 aliases
# ...
```


## Common args

From [1.10.0](/anomaly-detection/changelog#1100), **common args**, supported by *every model (and model type)* were introduced.

### Queries

Introduced in [1.10.0](/anomaly-detection/changelog#1100), as a part to support multi-model configs, `queries` arg is meant to define [queries from VmReader](/anomaly-detection/components/reader/?highlight=queries#config-parameters) particular model should be run on (meaning, all the series returned by each of these queries will be used in such model for fitting and inferencing).

`queries` arg is supported for all [the built-in](#built-in-models) (as well as for [custom](#custom-model-guide)) models.

This arg is **backward compatible** - if there is no explicit `queries` arg, then the model, defined in a config, will be run on ALL queries found in reader section:

```yaml
models:
  model_alias_1:
    # ...
    # no explicit `queries` arg is provided
```

will be implicitly converted to

```yaml
models:
  model_alias_1:
    # ...
    # if not set, `queries` arg is created and propagated with all query aliases found in `queries` arg of `reader` section
    queries: ['q1', 'q2', 'q3']  # i.e., if your `queries` in `reader` section has exactly q1, q2, q3 aliases
```

### Schedulers

Introduced in [1.11.0](/anomaly-detection/changelog#1110), as a part to support multi-scheduler configs, `schedulers` arg is meant to define [schedulers](/anomaly-detection/components/scheduler) particular model should be attached to.

`schedulers` arg is supported for all [the built-in](#built-in-models) (as well as for [custom](#custom-model-guide)) models.

This arg is **backward compatible** - if there is no explicit `schedulers` arg, then the model, defined in a config, will be attached to ALL the schedulers found in scheduler section:

```yaml
models:
  model_alias_1:
    # ...
    # no explicit `schedulers` arg is provided
```

will be implicitly converted to

```yaml
models:
  model_alias_1:
    # ...
    # if not set, `schedulers` arg is created and propagated with all scheduler aliases found in `schedulers` section
    schedulers: ['s1', 's2', 's3']  # i.e., if your `schedulers` section has exactly s1, s2, s3 aliases
```

### Provide series

Introduced in [1.12.0](/anomaly-detection/changelog#1120), `provide_series` arg limit the [output generated](#vmanomaly-output) by `vmanomaly` for writing. I.e. if the model produces default output series `['anomaly_score', 'yhat', 'yhat_lower', 'yhat_upper']` by specifying `provide_series` section as below, you limit the data being written to only `['anomaly_score']` for each metric received as a subject to anomaly detection.

```yaml
models:
  model_alias_1:
    # ...
    provide_series: ['anomaly_score']  # only `anomaly_score` metric will be available for writing back to the database
```

**Note** If `provide_series` is not specified in model config, the model will produce its default [model-dependent output](#vmanomaly-output). The output can't be less than `['anomaly_score']`. Even if `timestamp` column is omitted, it will be implicitly added to `provide_series` list, as it's required for metrics to be properly written.

### Detection direction
Introduced in [1.13.0](/anomaly-detection/CHANGELOG/#1130), `detection_direction` arg can help in reducing the number of [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/index.html#false-positive) and increasing the accuracy, when domain knowledge suggest to identify anomalies occurring when actual values (`y`) are *above, below, or in both directions* relative to the expected values (`yhat`). Available choices are: `both`, `above_expected`, `below_expected`.

Here's how default (backward-compatible) behavior looks like - anomalies will be tracked in `both` directions (`y > yhat` or `y < yhat`). This is useful when there is no domain expertise to filter the required direction.

<img src="/anomaly-detection/components/schema_detection_direction=both.webp" width="800px" alt="schema_detection_direction=both"/>

When set to `above_expected`, anomalies are tracked only when `y > yhat`.

*Example metrics*: Error rate, response time, page load time, number of failed transactions - metrics where *lower values are better*, so **higher** values are typically tracked.

<img src="/anomaly-detection/components/schema_detection_direction=above_expected.webp" width="800px" alt="schema_detection_direction=above_expected"/>

When set to `below_expected`, anomalies are tracked only when `y < yhat`. 

*Example metrics*: Service Level Agreement (SLA) compliance, conversion rate, Customer Satisfaction Score (CSAT) - metrics where *higher values are better*, so **lower** values are typically tracked.

<img src="/anomaly-detection/components/schema_detection_direction=below_expected.webp" width="800px" alt="schema_detection_direction=below_expected"/>

Config with a split example:

```yaml
models:
  model_above_expected:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3.0
    # track only cases when y > yhat, otherwise anomaly_score would be explicitly set to 0
    detection_direction: 'above_expected'
    # for this query we do not need to track lower values, thus, set anomaly detection tracking for y > yhat (above_expected)
    queries: ['query_values_the_lower_the_better']
  model_below_expected:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3.0
    # track only cases when y < yhat, otherwise anomaly_score would be explicitly set to 0
    detection_direction: 'below_expected'
    # for this query we do not need to track higher values, thus, set anomaly detection tracking for y < yhat (above_expected)
    queries: ['query_values_the_higher_the_better']
  model_bidirectional_default:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3.0
    # track in both direction, same backward-compatible behavior in case this arg is missing
    detection_direction: 'both'
    # for this query both directions can be equally important for anomaly detection, thus, setting it bidirectional (both)
    queries: ['query_values_both_direction_matters']
reader:
  # ...
  queries:
    query_values_the_lower_the_better: metricsql_expression1
    query_values_the_higher_the_better: metricsql_expression2
    query_values_both_direction_matters: metricsql_expression3
# other components like writer, schedule, monitoring
```

### Minimal deviation from expected

Introduced in [v1.13.0](/anomaly-detection/CHANGELOG/#1130), the `min_dev_from_expected` argument is designed to **reduce [false positives](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#false-positive)** in scenarios where deviations between the actual value (`y`) and the expected value (`yhat`) are **relatively** high. Such deviations can cause models to generate high [anomaly scores](/anomaly-detection/faq/#what-is-anomaly-score). However, these deviations may not be significant enough in **absolute values** from a business perspective to be considered anomalies. This parameter ensures that anomaly scores for data points where `|y - yhat| < min_dev_from_expected` are explicitly set to 0. By default, if this parameter is not set, it behaves as `min_dev_from_expected=0` to maintain backward compatibility.

> **Note**: `min_dev_from_expected` must be >= 0. The higher the value of `min_dev_from_expected`, the fewer data points will be available for anomaly detection, and vice versa.

*Example*: Consider a scenario where CPU utilization is low and oscillates around 0.3% (0.003). A sudden spike to 1.3% (0.013) represents a +333% increase in **relative** terms, but only a +1 percentage point (0.01) increase in **absolute** terms, which may be negligible and not warrant an alert. Setting the `min_dev_from_expected` argument to `0.01` (1%) will ensure that all anomaly scores for deviations <= `0.01` are set to 0.

Visualizations below demonstrate this concept; the green zone defined as the `[yhat - min_dev_from_expected, yhat + min_dev_from_expected]` range excludes actual data points (`y`) from generating anomaly scores if they fall within that range.

<img src="/anomaly-detection/components/schema_min_dev_from_expected=0.webp" width="800px" alt="min_dev_from_expected-default"/>

<img src="/anomaly-detection/components/schema_min_dev_from_expected=1.0.webp" width="800px" alt="min_dev_from_expected-small"/>

<img src="/anomaly-detection/components/schema_min_dev_from_expected=5.0.webp" width="800px" alt="min_dev_from_expected-big"/>

Example config of how to use this param based on query results:

```yaml
# other components like writer, schedulers, monitoring ...
reader:
  # ...
  queries:
    # the usage of min_dev should reduce false positives here
    need_to_include_min_dev: small_abs_values_metricsql_expression
    # min_dev is not really needed here
    normal_behavior: no_need_to_exclude_small_deviations_metricsql_expression
models:
  zscore_with_min_dev:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3
    min_dev_from_expected: 5.0
    queries: ['need_to_include_min_dev']  # use such models on queries where domain experience confirm usefulness
  zscore_wo_min_dev:
    class: 'zscore' # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3
    # if not set, equals to setting min_dev_from_expected == 0
    queries: ['normal_behavior']  # use the default where it's not needed
```  

## Model types

There are **2 model types**, supported in `vmanomaly`, resulting in **4 possible combinations**:

- [Univariate models](#univariate-models)
- [Multivariate models](#multivariate-models)

Each of these models can also be
- [Rolling](#rolling-models)
- [Non-rolling](#non-rolling-models)

### Univariate Models

For a univariate type, **one separate model** is fit/used for inference per **each time series**, defined in its [queries](#queries) arg.

For example, if you have some **univariate** model, defined to use 3 [MetricQL queries](https://docs.victoriametrics.com/metricsql/), each returning 5 time series, there will be 3*5=15 models created in total. Each such model produce **individual [output](#vmanomaly-output)** for each of time series. 

If during an inference, you got a series having **new labelset** (not present in any of fitted models), the inference will be skipped until you get a model, trained particularly for such labelset during forthcoming re-fit step.

**Implications:** Univariate models are a go-to default, when your queries returns **changing** amount of **individual** time series of **different** magnitude, [trend](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) or [seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality), so you won't be mixing incompatible data with different behavior within a single fit model (context isolation).

**Examples:** [Prophet](#prophet), [Holt-Winters](#holt-winters)

<p></p>
<img alt="vmanomaly-model-type-univariate" src="/anomaly-detection/components/model-lifecycle-univariate.webp" width="800px"/>

### Multivariate Models

For a multivariate type, **one shared model** is fit/used for inference on **all time series** simultaneously, defined in its [queries](#queries) arg. 

For example, if you have some **multivariate** model to use 3 [MetricQL queries](https://docs.victoriametrics.com/metricsql/), each returning 5 time series, there will be one shared model created in total. Once fit, this model will expect **exactly 15 time series with exact same labelsets as an input**. This model will produce **one shared [output](#vmanomaly-output)**.

If during an inference, you got a **different amount of series** or some series having a **new labelset** (not present in any of fitted models), the inference will be skipped until you get a model, trained particularly for such labelset during forthcoming re-fit step. 

**Implications:** Multivariate models are a go-to default, when your queries returns **fixed** amount of **individual** time series (say, some aggregations), to be used for adding cross-series (and cross-query) context, useful for catching [collective anomalies](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/#collective-anomalies) or [novelties](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/#novelties) (expanded to multi-input scenario). For example, you may set it up for anomaly detection of CPU usage in different modes (`idle`, `user`, `system`, etc.) and use its cross-dependencies to detect **unseen (in fit data)** behavior.

**Examples:** [IsolationForest](#isolation-forest-multivariate)

<p></p>
<img alt="vmanomaly-model-type-multivariate" src="/anomaly-detection/components/model-lifecycle-multivariate.webp" width="800px"/>

### Rolling Models

A rolling model is a model that, once trained, **cannot be (naturally) used to make inference on data, not seen during its fit phase**.

An instance of rolling model is **simultaneously fit and used for inference** during its `infer` method call.

As a result, such model instances are **not stored** between consecutive re-fit calls (defined by `fit_every` [arg](/anomaly-detection/components/scheduler/?highlight=fit_every#periodic-scheduler) in `PeriodicScheduler`), leading to **lower RAM** consumption.

Such models put **more pressure** on your reader's source, i.e. if your model should be fit on large amount of data (say, 14 days with 1-minute resolution) and at the same time you have **frequent inference** (say, once per minute) on new chunks of data - that's because such models require (fit + infer) window of data to be fit first to be used later in each inference call.

> **Note**: Rolling models require `fit_every` to be set equal to `infer_every` in your [PeriodicScheduler](/anomaly-detection/components/scheduler/?highlight=fit_every#periodic-scheduler).

**Examples:** [RollingQuantile](#rolling-quantile)

<p></p>
<img alt="vmanomaly-model-type-rolling" src="/anomaly-detection/components/model-type-rolling.webp" width="800px"/>

### Non-Rolling Models

Everything that is not classified as [rolling](#rolling-models). 

Produced models can be explicitly used to **infer on data, not seen during its fit phase**, thus, it **doesn't require re-fit procedure**.

Such models put **less pressure** on your reader's source, i.e. if you fit on large amount of data (say, 14 days with 1-minute resolution) but do it occasionally (say, once per day), at the same time you have **frequent inference**(say, once per minute) on new chunks of data

> **Note**: However, it's still highly recommended, to keep your model up-to-date with tendencies found in your data as it evolves in time.

Produced model instances are **stored in-memory** between consecutive re-fit calls (defined by `fit_every` [arg](/anomaly-detection/components/scheduler/?highlight=fit_every#periodic-scheduler) in `PeriodicScheduler`), leading to **higher RAM** consumption.

**Examples:** [Prophet](#prophet)

<p></p>
<img alt="vmanomaly-model-type-non-rolling" src="/anomaly-detection/components/model-type-non-rolling.webp" width="800px"/>

## Built-in Models 

### Overview
VM Anomaly Detection (`vmanomaly` hereinafter) models support 2 groups of parameters:

- **`vmanomaly`-specific** arguments - please refer to *Parameters specific for vmanomaly* and *Default model parameters* subsections for each of the models below.
- Arguments to **inner model** (say, [Facebook's Prophet](https://facebook.github.io/prophet/docs/quick_start.html#python-api)), passed in a `args` argument as key-value pairs, that will be directly given to the model during initialization to allow granular control. Optional.

**Note**: For users who may not be familiar with Python data types such as `list[dict]`, a [dictionary](https://www.w3schools.com/python/python_dictionaries.asp) in Python is a data structure that stores data values in key-value pairs. This structure allows for efficient data retrieval and management.


**Models**:
* [AutoTuned](#autotuned) - designed to take the cognitive load off the user, allowing any of built-in models below to be re-tuned for best params on data seen during each `fit` phase of the algorithm. Tradeoff is between increased computational time and optimized results / simpler maintenance.
* [Prophet](#prophet) - the most versatile one for production usage, especially for complex data ([trends](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend), [change points](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/#novelties), [multi-seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality))
* [Z-score](#z-score) - useful for testing and for simpler data ([de-trended](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) data without strict [seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality) and with anomalies of similar magnitude as your "normal" data)
* [Holt-Winters](#holt-winters) - well-suited for **data with moderate complexity**, exhibiting distinct [trends](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) and/or [seasonal patterns](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality).
* [MAD (Median Absolute Deviation)](#mad-median-absolute-deviation) - similarly to Z-score, is effective for **identifying outliers in relatively consistent data** (useful for detecting sudden, stark deviations from the median)
* [Rolling Quantile](#rolling-quantile) - best for **data with evolving patterns**, as it adapts to changes over a rolling window.
* [Seasonal Trend Decomposition](#seasonal-trend-decomposition) - similarly to Holt-Winters, is best for **data with pronounced [seasonal](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality) and [trend](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) components**
* [Isolation forest (Multivariate)](#isolation-forest-multivariate) - useful for **metrics data interaction** (several queries/metrics -> single anomaly score) and **efficient in detecting anomalies in high-dimensional datasets**
* [Custom model](#custom-model-guide) - benefit from your own models and expertise to better support your **unique use case**.


### AutoTuned
Tuning hyperparameters of a model can be tricky and often requires in-depth knowledge of Machine Learning. `AutoTunedModel` is designed specifically to take the cognitive load off the user - specify as little as `anomaly_percentage` param from `(0, 0.5)` interval and `tuned_model_class` (i.e. [`model.zscore.ZscoreModel`](/anomaly-detection/components/models/#z-score)) to get it working with best settings that match your data.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.auto.AutoTunedModel"` (or `auto` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `tuned_class_name` (string) - Built-in model class to tune, i.e. `model.zscore.ZscoreModel` (or `zscore` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support).
* `optimization_params` (dict) - Optimization parameters for unsupervised model tuning. Control % of found anomalies, as well as a tradeoff between time spent and the accuracy. The more `timeout` and `n_trials` are, the better model configuration can be found for `tuned_class_name`, but the longer it takes and vice versa. Set `n_jobs` to `-1` to use all the CPUs available, it makes sense if only you have a big dataset to train on during `fit` calls, otherwise overhead isn't worth it.
  - `anomaly_percentage` (float) - expected percentage of anomalies that can be seen in training data, from (0, 0.5) interval.
  - `seed` (int) - Random seed for reproducibility and deterministic nature of underlying optimizations.
  - `n_splits` (int) - How many folds to create for hyperparameter tuning out of your data. The higher, the longer it takes but the better the results can be. Defaults to 3.
  - `n_trials` (int) - How many trials to sample from hyperparameter search space. The higher, the longer it takes but the better the results can be. Defaults to 128.
  - `timeout` (float) - How many seconds in total can be spent on each model to tune hyperparameters. The higher, the longer it takes, allowing to test more trials out of defined `n_trials`, but the better the results can be.

<img alt="vmanomaly-autotune-schema" src="/anomaly-detection/components/autotune.webp" width="800px"/>

```yaml
# ...
models:
  your_desired_alias_for_a_model:
    class: 'auto'  # or 'model.auto.AutoTunedModel' until v1.13.0
    tuned_class_name: 'zscore'  # or 'model.zscore.ZscoreModel' until v1.13.0
    optimization_params:
      anomaly_percentage: 0.004  # required. i.e. we expect <= 0.4% of anomalies to be present in training data
      seed: 42  # fix reproducibility & determinism
      n_splits: 4  # how much folds are created for internal cross-validation
      n_trials: 128  # how many configurations to sample from search space during optimization
      timeout: 10  # how many seconds to spend on optimization for each trained model during `fit` phase call
      n_jobs: 1  # how many jobs in parallel to launch. Consider making it > 1 only if you have fit window containing > 10000 datapoints for each series
  # ...
```

**Note**: Autotune can't be made on your [custom model](#custom-model-guide). Also, it can't be applied to itself (like `tuned_class_name: 'model.auto.AutoTunedModel'`)


### [Prophet](https://facebook.github.io/prophet/)
Here we utilize the Facebook Prophet implementation, as detailed in their [library documentation](https://facebook.github.io/prophet/docs/quick_start.html#python-api). All parameters from this library are compatible and can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.prophet.ProphetModel"` (or `prophet` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `seasonalities` (list[dict], optional) - Extra seasonalities to pass to Prophet. See [`add_seasonality()`](https://facebook.github.io/prophet/docs/seasonality,_holiday_effects,_and_regressors.html#modeling-holidays-and-special-events:~:text=modeling%20the%20cycle-,Specifying,-Custom%20Seasonalities) Prophet param.

**Note**: Apart from standard `vmanomaly` output, Prophet model can provide [additional metrics](#additional-output-metrics-produced-by-fb-prophet).

**Additional output metrics produced by FB Prophet**
Depending on chosen `seasonality` parameter FB Prophet can return additional metrics such as:
- `trend`, `trend_lower`, `trend_upper`
- `additive_terms`, `additive_terms_lower`, `additive_terms_upper`,
- `multiplicative_terms`, `multiplicative_terms_lower`, `multiplicative_terms_upper`,
- `daily`, `daily_lower`, `daily_upper`,
- `hourly`, `hourly_lower`, `hourly_upper`,
- `holidays`, `holidays_lower`, `holidays_upper`,
- and a number of columns for each holiday if `holidays` param is set

*Config Example*

```yaml
models:
  your_desired_alias_for_a_model:
    class: 'prophet'  # or 'model.prophet.ProphetModel' until v1.13.0
    provide_series: ['anomaly_score', 'yhat', 'yhat_lower', 'yhat_upper', 'trend']
    seasonalities:
      - name: 'hourly'
        period: 0.04166666666
        fourier_order: 30
    # Inner model args (key-value pairs) accepted by
    # https://facebook.github.io/prophet/docs/quick_start.html#python-api
    args:
      # See https://facebook.github.io/prophet/docs/uncertainty_intervals.html
      interval_width: 0.98
      country_holidays: 'US'
```


Resulting metrics of the model are described [here](#vmanomaly-output)

### [Z-score](https://en.wikipedia.org/wiki/Standard_score)
*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.zscore.ZscoreModel"` (or `zscore` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculation boundaries and anomaly score. Defaults to `2.5`.

*Config Example*


```yaml
models:
  your_desired_alias_for_a_model:
    class: "zscore"  # or 'model.zscore.ZscoreModel' until v1.13.0
    z_threshold: 3.5
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [Holt-Winters](https://en.wikipedia.org/wiki/Exponential_smoothing)
Here we use Holt-Winters Exponential Smoothing implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.holtwinters.HoltWinters"` (or `holtwinters` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)

* `frequency` (string) - Must be set equal to sampling_period. Model needs to know expected data-points frequency (e.g. '10m'). If omitted, frequency is guessed during fitting as **the median of intervals between fitting data timestamps**. During inference, if incoming data doesn't have the same frequency, then it will be interpolated.  E.g. data comes at 15 seconds resolution, and our resample_freq is '1m'. Then fitting data will be downsampled to '1m' and internal model is trained at '1m' intervals. So, during inference, prediction data would be produced at '1m' intervals, but interpolated to "15s" to match with expected output, as output data must have the same timestamps. As accepted by pandas.Timedelta (e.g. '5m').

* `seasonality` (string, optional) - As accepted by pandas.Timedelta.

* If `seasonal_periods` is not specified, it is calculated as `seasonality` / `frequency`
Used to compute "seasonal_periods" param for the model (e.g. '1D' or '1W').

* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to 2.5.


*Default model parameters*:

* If [parameter](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters) `seasonal` is not specified, default value will be `add`.

* If [parameter](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters) `initialization_method` is not specified, default value will be `estimated`.

* `args` (dict, optional) - Inner model args (key-value pairs). See accepted params in [model documentation](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters). Defaults to empty (not provided). Example:  {"seasonal": "add", "initialization_method": "estimated"}

*Config Example*

```yaml
models:
  your_desired_alias_for_a_model:
    class: "holtwinters"  # or 'model.holtwinters.HoltWinters' until v1.13.0
    seasonality: '1d'
    frequency: '1h'
    # Inner model args (key-value pairs) accepted by statsmodels.tsa.holtwinters.ExponentialSmoothing
    args:
      seasonal: 'add'
      initialization_method: 'estimated'
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [MAD (Median Absolute Deviation)](https://en.wikipedia.org/wiki/Median_absolute_deviation)
The MAD model is a robust method for anomaly detection that is *less sensitive* to outliers in data compared to standard deviation-based models. It considers a point as an anomaly if the absolute deviation from the median is significantly large.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.mad.MADModel"` (or `mad` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `threshold` (float, optional) - The threshold multiplier for the MAD to determine anomalies. Defaults to `2.5`. Higher values will identify fewer points as anomalies.

*Config Example*


```yaml
models:
  your_desired_alias_for_a_model:
    class: "mad"  # or 'model.mad.MADModel' until v1.13.0
    threshold: 2.5
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [Rolling Quantile](https://en.wikipedia.org/wiki/Quantile)

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.rolling_quantile.RollingQuantileModel"` (or `rolling_quantile` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `quantile` (float) - quantile value, from 0.5 to 1.0. This constraint is implied by 2-sided confidence interval.
* `window_steps` (integer) - size of the moving window. (see 'sampling_period')

*Config Example*

```yaml
models:
  your_desired_alias_for_a_model:
    class: "rolling_quantile" # or 'model.rolling_quantile.RollingQuantileModel' until v1.13.0
    quantile: 0.9
    window_steps: 96
```

Resulting metrics of the model are described [here](#vmanomaly-output).

### [Seasonal Trend Decomposition](https://en.wikipedia.org/wiki/Seasonal_adjustment)
Here we use Seasonal Decompose implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.seasonal.seasonal_decompose.html). Parameters from this library can be passed to the model. Some parameters are specifically predefined in `vmanomaly` and can't be changed by user(`model`='additive', `two_sided`=False).

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.std.StdModel"` (or `std` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)
* `period` (integer) -  Number of datapoints in one season.
* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to `2.5`.


*Config Example*


```yaml
models:
  your_desired_alias_for_a_model:
    class: "std"  # or 'model.std.StdModel' starting from v1.13.0
    period: 2
```


Resulting metrics of the model are described [here](#vmanomaly-output).

**Additional output metrics produced by Seasonal Trend Decomposition model**
* `resid` - The residual component of the data series.
* `trend` - The trend component of the data series.
* `seasonal` - The seasonal component of the data series.


### [Isolation forest](https://en.wikipedia.org/wiki/Isolation_forest) (Multivariate)
Detects anomalies using binary trees. The algorithm has a linear time complexity and a low memory requirement, which works well with high-volume data. It can be used on both univariate and multivariate data, but it is more effective in multivariate case.

**Important**: Be aware of [the curse of dimensionality](https://en.wikipedia.org/wiki/Curse_of_dimensionality). Don't use single multivariate model if you expect your queries to return many time series of less datapoints that the number of metrics. In such case it is hard for a model to learn meaningful dependencies from too sparse data hypercube.

Here we use Isolation Forest implementation from `scikit-learn` [library](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.isolation_forest.IsolationForestMultivariateModel"` (or `isolation_forest_multivariate` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support)

* `contamination` (float or string, optional) - The amount of contamination of the data set, i.e. the proportion of outliers in the data set. Used when fitting to define the threshold on the scores of the samples. Default value - "auto". Should be either `"auto"` or be in the range (0.0, 0.5].

* `seasonal_features` (list of string) - List of seasonality to encode through [cyclical encoding](https://towardsdatascience.com/cyclical-features-encoding-its-about-time-ce23581845ca), i.e. `dow` (day of week). **Introduced in [1.12.0](/anomaly-detection/CHANGELOG/#v1120)**. 
  - Empty by default for backward compatibility.
  - Example: `seasonal_features: ['dow', 'hod']`.
  - Supported seasonalities:
    - "minute" - minute of hour (0-59)
    - "hod" - hour of day (0-23)
    - "dow" - day of week (1-7)
    - "month" - month of year (1-12)

* `args` (dict, optional) - Inner model args (key-value pairs). See accepted params in [model documentation](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). Defaults to empty (not provided). Example:  {"random_state": 42, "n_estimators": 100}

*Config Example*


```yaml
models:
  your_desired_alias_for_a_model:
    # To use univariate model, substitute class argument with "model.isolation_forest.IsolationForestModel".
    class: "isolation_forest_multivariate" # or 'model.isolation_forest.IsolationForestMultivariateModel' until v1.13.0
    contamination: "0.01"
    provide_series: ['anomaly_score']
    seasonal_features: ['dow', 'hod']
    args:
      n_estimators: 100
      # i.e. to assure reproducibility of produced results each time model is fit on the same input
      random_state: 42
```


Resulting metrics of the model are described [here](#vmanomaly-output).

## vmanomaly output

When `vmanomaly` is executed, it generates various metrics, the specifics of which depend on the model employed.
These metrics can be renamed in the writer's section.

The default metrics produced by `vmanomaly` include:

- `anomaly_score`: This is the *primary* metric.
    - It is designed in such a way that values from 0.0 to 1.0 indicate non-anomalous data.
    - A value greater than 1.0 is generally classified as an anomaly, although this threshold can be adjusted in the alerting configuration.
    - The decision to set the changepoint at 1 was made to ensure consistency across various models and alerting configurations, such that a score above 1 consistently signifies an anomaly.

- `yhat`: This represents the predicted expected value.

- `yhat_lower`: This indicates the predicted lower boundary.

- `yhat_upper`: This refers to the predicted upper boundary.

- `y`: This is the original value obtained from the query result.

**Important**: Be aware that if `NaN` (Not a Number) or `Inf` (Infinity) values are present in the input data during `infer` model calls, the model will produce `NaN` as the `anomaly_score` for these particular instances.


## vmanomaly monitoring metrics

Each model exposes [several monitoring metrics](/anomaly-detection/components/monitoring/#models-behaviour-metrics) to its `health_path` endpoint:


## Custom Model Guide

Apart from `vmanomaly` [built-in models](/anomaly-detection/components/models/#built-in-models), users can create their own custom models for anomaly detection.

Here in this guide, we will
- Make a file containing our custom model definition
- Define VictoriaMetrics Anomaly Detection config file to use our custom model
- Run service

**Note**: The file containing the model should be written in [Python language](https://www.python.org/) (3.11+)

### 1. Custom model

> **Note**: By default, each custom model is created as [**univariate**](#univariate-models) / [**non-rolling**](#non-rolling-models) model. If you want to override this behavior, define models inherited from `RollingModel` (to get a rolling model), or having `is_multivariate` class arg set to `True` (please refer to the code example below).

We'll create `custom_model.py` file with `CustomModel` class that will inherit from `vmanomaly`'s `Model` base class.
In the `CustomModel` class, the following methods are required: - `__init__`, `fit`, `infer`, `serialize` and `deserialize`:
* `__init__` method should initiate parameters for the model.

  **Note**: if your model relies on configs that have `arg` [key-value pair argument](./models.md#section-overview), do not forget to use Python's `**kwargs` in method's signature and to explicitly call

  ```python 
  super().__init__(**kwargs)
  ``` 
  to initialize the base class each model derives from
* `fit` method should contain the model training process. Please be aware that for `RollingModel` defining `fit` method is not needed, as the whole fit/infer process should be defined completely in `infer` method.
* `infer` should return Pandas.DataFrame object with model's inferences.
* `serialize` method that saves the model on disk.
* `deserialize` load the saved model from disk.

For the sake of simplicity, the model in this example will return one of two values of `anomaly_score` - 0 or 1 depending on input parameter `percentage`.


```python
import numpy as np
import pandas as pd
import scipy.stats as st
import logging
from pickle import dumps

from model.model import (
  PICKLE_PROTOCOL,
  Model,
  deserialize_basic
)
# from model.model import RollingModel  # inherit from it for your model to be of rolling type
logger = logging.getLogger(__name__)


class CustomModel(Model):
  """
  Custom model implementation.
  """
  # by default, each `Model` will be created as a univariate one
  # uncomment line below for it to be of multivariate type
  #`is_multivariate = True`
  
  def __init__(self, percentage: float = 0.95, **kwargs):
    super().__init__(**kwargs)
    self.percentage = percentage
    self._mean = np.nan
    self._std = np.nan

  def fit(self, df: pd.DataFrame):
    # Model fit process:
    y = df['y']
    self._mean = np.mean(y)
    self._std = np.std(y)
    if self._std == 0.0:
      self._std = 1 / 65536

  def infer(self, df: pd.DataFrame) -> np.array:
    # Inference process:
    y = df['y']
    zscores = (y - self._mean) / self._std
    anomaly_score_cdf = st.norm.cdf(np.abs(zscores))
    df_pred = df[['timestamp', 'y']].copy()
    df_pred['anomaly_score'] = anomaly_score_cdf > self.percentage
    df_pred['anomaly_score'] = df_pred['anomaly_score'].astype('int32', errors='ignore')

    return df_pred

    def serialize(self) -> None:
      return dumps(self, protocol=PICKLE_PROTOCOL)

    @staticmethod
    def deserialize(model: str | bytes) -> 'CustomModel':
      return deserialize_basic(model)
```


### 2. Configuration file

Next, we need to create `config.yaml` file with `vmanomaly` configuration and model input parameters.
In the config file's `models` section we need to set our model class to `model.custom.CustomModel` (or `custom` starting from [v1.13.0](/anomaly-detection/CHANGELOG/#1130) with class alias support) and define all parameters used in `__init__` method.
You can find out more about configuration parameters in `vmanomaly` [config docs](/anomaly-detection/components/).

```yaml
schedulers:
  s1:
    infer_every: "1m"
    fit_every: "1m"
    fit_window: "1d"

models:
  custom_model:
    class: "custom"  # or 'model.model.CustomModel' until v1.13.0
    percentage: 0.9


reader:
  datasource_url: "http://victoriametrics:8428/"
  sampling_period: '1m'
  queries:
    ingestion_rate: 'sum(rate(vm_rows_inserted_total)) by (type)'
    churn_rate: 'sum(rate(vm_new_timeseries_created_total[5m]))'

writer:
  datasource_url: "http://victoriametrics:8428/"
  metric_format:
    __name__: "custom_$VAR"
    for: "$QUERY_KEY"
    run: "test-format"

monitoring:
  # /metrics server.
  pull:
    port: 8080
  push:
    url: "http://victoriametrics:8428/"
    extra_labels:
      job: "vmanomaly-develop"
      config: "custom.yaml"
```


### 3. Running custom model
Let's pull the docker image for `vmanomaly`:

```sh 
docker pull victoriametrics/vmanomaly:latest
```

Now we can run the docker container putting as volumes both config and model file:

> **Note**: place the model file to `/model/custom.py` path when copying

./custom_model.py:/vmanomaly/model/custom.py

```sh
docker run -it \
-v $(PWD)/license:/license \
-v $(PWD)/custom_model.py:/vmanomaly/model/custom.py \
-v $(PWD)/custom.yaml:/config.yaml \
victoriametrics/vmanomaly:latest /config.yaml \
--license-file=/license
```

Please find more detailed instructions (license, etc.) [here](/anomaly-detection/overview.html#run-vmanomaly-docker-container)


### Output
As the result, this model will return metric with labels, configured previously in `config.yaml`.
In this particular example, 2 metrics will be produced. Also, there will be added other metrics from input query result.

```text
{__name__="custom_anomaly_score", for="ingestion_rate", model_alias="custom_model", scheduler_alias="s1", run="test-format"},
{__name__="custom_anomaly_score", for="churn_rate",     model_alias="custom_model", scheduler_alias="s1", run="test-format"}
```
