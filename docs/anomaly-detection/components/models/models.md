---
# sort: 1
weight: 1
title: Built-in Models
# disableToc: true
menu:
  docs:
    parent: "vmanomaly-models"
    # sort: 1
    weight: 1
aliases:
  - /anomaly-detection/components/models/models.html
---

# Models config parameters

## Section Overview
VM Anomaly Detection (`vmanomaly` hereinafter) models support 2 groups of parameters:

- **`vmanomaly`-specific** arguments - please refer to *Parameters specific for vmanomaly* and *Default model parameters* subsections for each of the models below.
- Arguments to **inner model** (say, [Facebook's Prophet](https://facebook.github.io/prophet/docs/quick_start.html#python-api)), passed in a `args` argument as key-value pairs, that will be directly given to the model during initialization to allow granular control. Optional.

**Note**: For users who may not be familiar with Python data types such as `list[dict]`, a [dictionary](https://www.w3schools.com/python/python_dictionaries.asp) in Python is a data structure that stores data values in key-value pairs. This structure allows for efficient data retrieval and management.


**Models**:
* [ARIMA](#arima)
* [Holt-Winters](#holt-winters) 
* [Prophet](#prophet)
* [Rolling Quantile](#rolling-quantile)
* [Seasonal Trend Decomposition](#seasonal-trend-decomposition)
* [Z-score](#z-score)
* [MAD (Median Absolute Deviation)](#mad-median-absolute-deviation)
* [Isolation forest (Multivariate)](#isolation-forest-multivariate)
* [Custom model](#custom-model)

---
## [ARIMA](https://en.wikipedia.org/wiki/Autoregressive_integrated_moving_average)
Here we use ARIMA implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.arima.model.ARIMA.html)

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.arima.ArimaModel"`

* `z_threshold` (float) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to 2.5.

* `provide_series` (list[string]) - List of columns to be produced and returned by the model. Defaults to `["anomaly_score", "yhat", "yhat_lower" "yhat_upper", "y"]`. Output can be **only a subset** of a given column list.

* `resample_freq` (string) = Frequency to resample input data into, e.g. data comes at 15 seconds resolution, and resample_freq is '1m'. Then fitting data will be downsampled to '1m' and internal model is trained at '1m' intervals. So, during inference, prediction data would be produced at '1m' intervals, but interpolated to "15s" to match with expected output, as output data must have the same timestamps.

*Default model parameters*:

* `order`\* (list[int]) - ARIMA's (p,d,q) order of the model for the autoregressive, differences, and moving average components, respectively.
    
* `args`: (dict) - Inner model args (key-value pairs). See accepted params in [model documentation](https://www.statsmodels.org/dev/generated/statsmodels.tsa.arima.model.ARIMA.html). Defaults to empty (not provided). Example:  {"trend": "c"}

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.arima.ArimaModel"
  # ARIMA's (p,d,q) order
  order: 
  - 1
  - 1
  - 0
  z_threshold: 2.7
  resample_freq: '1m'
  # Inner model args (key-value pairs) accepted by statsmodels.tsa.arima.model.ARIMA
  args:
    trend: 'c'
```
</div>

---
## [Holt-Winters](https://en.wikipedia.org/wiki/Exponential_smoothing)
Here we use Holt-Winters Exponential Smoothing implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.holtwinters.HoltWinters"`

* `frequency`\* (string) - Must be set equal to sampling_period. Model needs to know expected data-points frequency (e.g. '10m').
If omitted, frequency is guessed during fitting as **the median of intervals between fitting data timestamps**. During inference, if incoming data doesn't have the same frequency, then it will be interpolated.
        
E.g. data comes at 15 seconds resolution, and our resample_freq is '1m'. Then fitting data will be downsampled to '1m' and internal model is trained at '1m' intervals. So, during inference, prediction data would be produced at '1m' intervals, but interpolated to "15s" to match with expected output, as output data must have the same timestamps. 

As accepted by pandas.Timedelta (e.g. '5m').

* `seasonality` (string) - As accepted by pandas.Timedelta.
If `seasonal_periods` is not specified, it is calculated as `seasonality` / `frequency`
Used to compute "seasonal_periods" param for the model (e.g. '1D' or '1W').
        
* `z_threshold` (float) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to 2.5.


*Default model parameters*:

* If [parameter](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters) `seasonal` is not specified, default value will be `add`.

* If [parameter](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters) `initialization_method` is not specified, default value will be `estimated`.

* `args`: (dict) - Inner model args (key-value pairs). See accepted params in [model documentation](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html#statsmodels.tsa.holtwinters.ExponentialSmoothing-parameters). Defaults to empty (not provided). Example:  {"seasonal": "add", "initialization_method": "estimated"}

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.holtwinters.HoltWinters"
  seasonality: '1d'
  frequency: '1h'
  # Inner model args (key-value pairs) accepted by statsmodels.tsa.holtwinters.ExponentialSmoothing
  args:
    seasonal: 'add'
    initialization_method: 'estimated'
```
</div>

Resulting metrics of the model are described [here](#vmanomaly-output).

---
## [Prophet](https://facebook.github.io/prophet/)
Here we utilize the Facebook Prophet implementation, as detailed in their [library documentation](https://facebook.github.io/prophet/docs/quick_start.html#python-api). All parameters from this library are compatible and can be passed to the model.

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.prophet.ProphetModel"`
* `seasonalities` (list[dict]) - Extra seasonalities to pass to Prophet. See [`add_seasonality()`](https://facebook.github.io/prophet/docs/seasonality,_holiday_effects,_and_regressors.html#modeling-holidays-and-special-events:~:text=modeling%20the%20cycle-,Specifying,-Custom%20Seasonalities) Prophet param.
* `provide_series` - model resulting metrics. If not specified [standard metrics](#vmanomaly-output) will be provided. 

**Note**: Apart from standard vmanomaly output Prophet model can provide [additional metrics](#additional-output-metrics-produced-by-fb-prophet).

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
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.prophet.ProphetModel"
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
</div>

Resulting metrics of the model are described [here](#vmanomaly-output)

---
## [Rolling Quantile](https://en.wikipedia.org/wiki/Quantile)

*Parameters specific for vmanomaly*:

\* - mandatory parameters.

* `class`\* (string) - model class name `"model.rolling_quantile.RollingQuantileModel"`
* `quantile`\* (float) - quantile value, from 0.5 to 1.0. This constraint is implied by 2-sided confidence interval.
* `window_steps`\* (integer) - size of the moving window. (see 'sampling_period') 

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.rolling_quantile.RollingQuantileModel"
  quantile: 0.9
  window_steps: 96
```
</div>

Resulting metrics of the model are described [here](#vmanomaly-output).

---
## [Seasonal Trend Decomposition](https://en.wikipedia.org/wiki/Seasonal_adjustment)
Here we use Seasonal Decompose implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.seasonal.seasonal_decompose.html). Parameters from this library can be passed to the model. Some parameters are specifically predefined in vmanomaly and can't be changed by user(`model`='additive', `two_sided`=False).

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.std.StdModel"`
* `period`\* (integer) -  Number of datapoints in one season.
* `z_threshold` (float) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to 2.5.


*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.std.StdModel"
  period: 2
```
</div>

Resulting metrics of the model are described [here](#vmanomaly-output).

**Additional output metrics produced by Seasonal Trend Decomposition model**
* `resid` - The residual component of the data series.
* `trend` - The trend component of the data series.
* `seasonal` - The seasonal component of the data series.

---
## [MAD (Median Absolute Deviation)](https://en.wikipedia.org/wiki/Median_absolute_deviation)
The MAD model is a robust method for anomaly detection that is *less sensitive* to outliers in data compared to standard deviation-based models. It considers a point as an anomaly if the absolute deviation from the median is significantly large.

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.mad.MADModel"`
* `threshold` (float) - The threshold multiplier for the MAD to determine anomalies. Defaults to 2.5. Higher values will identify fewer points as anomalies.

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.mad.MADModel"
  threshold: 2.5
```
Resulting metrics of the model are described [here](#vmanomaly-output).

---
## [Z-score](https://en.wikipedia.org/wiki/Standard_score)
*Parameters specific for vmanomaly*:
\* - mandatory parameters.
* `class`\* (string) - model class name `"model.zscore.ZscoreModel"`
* `z_threshold` (float) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculation boundaries and anomaly score. Defaults to 2.5.

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  class: "model.zscore.ZscoreModel"
  z_threshold: 2.5
```
</div>

Resulting metrics of the model are described [here](#vmanomaly-output).

## [Isolation forest](https://en.wikipedia.org/wiki/Isolation_forest) (Multivariate)
Detects anomalies using binary trees. The algorithm has a linear time complexity and a low memory requirement, which works well with high-volume data. It can be used on both univatiate and multivariate data, but it is more effective in multivariate case.

**Important**: Be aware of [the curse of dimensionality](https://en.wikipedia.org/wiki/Curse_of_dimensionality). Don't use single multivariate model if you expect your queries to return many time series of less datapoints that the number of metrics. In such case it is hard for a model to learn meaningful dependencies from too sparse data hypercube.

Here we use Isolation Forest implementation from `scikit-learn` [library](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

\* - mandatory parameters.
* `class`\* (string) - model class name `"model.isolation_forest.IsolationForestMultivariateModel"`

* `contamination` - The amount of contamination of the data set, i.e. the proportion of outliers in the data set. Used when fitting to define the threshold on the scores of the samples. Default value - "auto". Should be either `"auto"` or be in the range (0.0, 0.5].

* `args`: (dict) - Inner model args (key-value pairs). See accepted params in [model documentation](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). Defaults to empty (not provided). Example:  {"random_state": 42, "n_estimators": 100}

*Config Example*
<div class="with-copy" markdown="1">

```yaml
model:
  # To use univariate model, substitute class argument with "model.isolation_forest.IsolationForestModel".
  class: "model.isolation_forest.IsolationForestMultivariateModel"
  contamination: "auto"
  args:
    n_estimators: 100
    # i.e. to assure reproducibility of produced results each time model is fit on the same input
    random_state: 42
```
</div>

Resulting metrics of the model are described [here](#vmanomaly-output).

---
## Custom model
You can find a guide on setting up a custom model [here](./custom_model.md).

## vmanomaly output

When vmanomaly is executed, it generates various metrics, the specifics of which depend on the model employed. 
These metrics can be renamed in the writer's section. 

The default metrics produced by vmanomaly include:

- `anomaly_score`: This is the *primary* metric. 
  - It is designed in such a way that values from 0.0 to 1.0 indicate non-anomalous data. 
  - A value greater than 1.0 is generally classified as an anomaly, although this threshold can be adjusted in the alerting configuration.
  - The decision to set the changepoint at 1 was made to ensure consistency across various models and alerting configurations, such that a score above 1 consistently signifies an anomaly.
  
- `yhat`: This represents the predicted expected value.

- `yhat_lower`: This indicates the predicted lower boundary.

- `yhat_upper`: This refers to the predicted upper boundary.

- `y`: This is the original value obtained from the query result.

**Important**: Be aware that if `NaN` (Not a Number) or `Inf` (Infinity) values are present in the input data during `infer` model calls, the model will produce `NaN` as the `anomaly_score` for these particular instances.


## Healthcheck metrics

Each model exposes [several healthchecks metrics](./../monitoring.html#models-behaviour-metrics) to its `health_path` endpoint: 