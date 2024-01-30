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

This section describes `Model` component of VictoriaMetrics Anomaly Detection (or simply [`vmanomaly`](/anomaly-detection/overview.html)) and the guide of how to define a respective section of a config to launch the service.
vmanomaly includes various [built-in models](#built-in-models) and you can integrate your custom model with vmanomaly see [custom model](#custom-model-guide) 


## Built-in Models 

### Overview
VM Anomaly Detection (`vmanomaly` hereinafter) models support 2 groups of parameters:

- **`vmanomaly`-specific** arguments - please refer to *Parameters specific for vmanomaly* and *Default model parameters* subsections for each of the models below.
- Arguments to **inner model** (say, [Facebook's Prophet](https://facebook.github.io/prophet/docs/quick_start.html#python-api)), passed in a `args` argument as key-value pairs, that will be directly given to the model during initialization to allow granular control. Optional.

**Note**: For users who may not be familiar with Python data types such as `list[dict]`, a [dictionary](https://www.w3schools.com/python/python_dictionaries.asp) in Python is a data structure that stores data values in key-value pairs. This structure allows for efficient data retrieval and management.


**Models**:

* [Prophet](#prophet) - the most versatile one for production usage, especially for complex data ([trends](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend), [change points](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/#novelties), [multi-seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality))
* [Z-score](#z-score) - useful for testing and for simpler data ([de-trended](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) data without strict [seasonality](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality) and with anomalies of similar magnitude as your "normal" data)
* [Holt-Winters](#holt-winters) - well-suited for **data with moderate complexity**, exhibiting distinct [trends](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) and/or [seasonal patterns](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality).
* [MAD (Median Absolute Deviation)](#mad-median-absolute-deviation) - similarly to Z-score, is effective for **identifying outliers in relatively consistent data** (useful for detecting sudden, stark deviations from the median)
* [Rolling Quantile](#rolling-quantile) - best for **data with evolving patterns**, as it adapts to changes over a rolling window.
* [Seasonal Trend Decomposition](#seasonal-trend-decomposition) - similarly to Holt-Winters, is best for **data with pronounced [seasonal](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#seasonality) and [trend](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-1/#trend) components**
* [ARIMA](#arima) - use when your data shows **clear patterns or autocorrelation (the degree of correlation between values of the same series at different periods)**. However, good understanding of machine learning is required to tune.
* [Isolation forest (Multivariate)](#isolation-forest-multivariate) - useful for **metrics data interaction** (several queries/metrics -> single anomaly score) and **efficient in detecting anomalies in high-dimensional datasets**
* [Custom model](#custom-model-guide) - benefit from your own models and expertise to better support your **unique use case**.


### [Prophet](https://facebook.github.io/prophet/)
Here we utilize the Facebook Prophet implementation, as detailed in their [library documentation](https://facebook.github.io/prophet/docs/quick_start.html#python-api). All parameters from this library are compatible and can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.prophet.ProphetModel"`
* `seasonalities` (list[dict], optional) - Extra seasonalities to pass to Prophet. See [`add_seasonality()`](https://facebook.github.io/prophet/docs/seasonality,_holiday_effects,_and_regressors.html#modeling-holidays-and-special-events:~:text=modeling%20the%20cycle-,Specifying,-Custom%20Seasonalities) Prophet param.
* `provide_series` (dict, optional) - model resulting metrics. If not specified [standard metrics](#vmanomaly-output) will be provided.

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


Resulting metrics of the model are described [here](#vmanomaly-output)

### [Z-score](https://en.wikipedia.org/wiki/Standard_score)
*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.zscore.ZscoreModel"`
* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculation boundaries and anomaly score. Defaults to `2.5`.

*Config Example*


```yaml
model:
  class: "model.zscore.ZscoreModel"
  z_threshold: 2.5
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [Holt-Winters](https://en.wikipedia.org/wiki/Exponential_smoothing)
Here we use Holt-Winters Exponential Smoothing implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.holtwinters.HoltWinters"`

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
model:
  class: "model.holtwinters.HoltWinters"
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

* `class` (string) - model class name `"model.mad.MADModel"`
* `threshold` (float, optional) - The threshold multiplier for the MAD to determine anomalies. Defaults to `2.5`. Higher values will identify fewer points as anomalies.

*Config Example*


```yaml
model:
  class: "model.mad.MADModel"
  threshold: 2.5
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [Rolling Quantile](https://en.wikipedia.org/wiki/Quantile)

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.rolling_quantile.RollingQuantileModel"`
* `quantile` (float) - quantile value, from 0.5 to 1.0. This constraint is implied by 2-sided confidence interval.
* `window_steps` (integer) - size of the moving window. (see 'sampling_period')

*Config Example*

```yaml
model:
  class: "model.rolling_quantile.RollingQuantileModel"
  quantile: 0.9
  window_steps: 96
```


Resulting metrics of the model are described [here](#vmanomaly-output).

### [Seasonal Trend Decomposition](https://en.wikipedia.org/wiki/Seasonal_adjustment)
Here we use Seasonal Decompose implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.seasonal.seasonal_decompose.html). Parameters from this library can be passed to the model. Some parameters are specifically predefined in vmanomaly and can't be changed by user(`model`='additive', `two_sided`=False).

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.std.StdModel"`
* `period` (integer) -  Number of datapoints in one season.
* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to `2.5`.


*Config Example*


```yaml
model:
  class: "model.std.StdModel"
  period: 2
```


Resulting metrics of the model are described [here](#vmanomaly-output).

**Additional output metrics produced by Seasonal Trend Decomposition model**
* `resid` - The residual component of the data series.
* `trend` - The trend component of the data series.
* `seasonal` - The seasonal component of the data series.

### [ARIMA](https://en.wikipedia.org/wiki/Autoregressive_integrated_moving_average)
Here we use ARIMA implementation from `statsmodels` [library](https://www.statsmodels.org/dev/generated/statsmodels.tsa.arima.model.ARIMA.html)

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.arima.ArimaModel"`

* `z_threshold` (float, optional) - [standard score](https://en.wikipedia.org/wiki/Standard_score) for calculating boundaries to define anomaly score. Defaults to `2.5`.

* `provide_series` (list[string], optional) - List of columns to be produced and returned by the model. Defaults to `["anomaly_score", "yhat", "yhat_lower" "yhat_upper", "y"]`. Output can be **only a subset** of a given column list.

* `resample_freq` (string, optional) - Frequency to resample input data into, e.g. data comes at 15 seconds resolution, and resample_freq is '1m'. Then fitting data will be downsampled to '1m' and internal model is trained at '1m' intervals. So, during inference, prediction data would be produced at '1m' intervals, but interpolated to "15s" to match with expected output, as output data must have the same timestamps.

*Default model parameters*:

* `order` (list[int]) - ARIMA's (p,d,q) order of the model for the autoregressive, differences, and moving average components, respectively.

* `args` (dict, optional) - Inner model args (key-value pairs). See accepted params in [model documentation](https://www.statsmodels.org/dev/generated/statsmodels.tsa.arima.model.ARIMA.html). Defaults to empty (not provided). Example:  {"trend": "c"}

*Config Example*

```yaml
model:
  class: "model.arima.ArimaModel"
  # ARIMA's (p,d,q) order
  order: [1, 1, 0] 
  z_threshold: 2.7
  resample_freq: '1m'
  # Inner model args (key-value pairs) accepted by statsmodels.tsa.arima.model.ARIMA
  args:
    trend: 'c'
```


### [Isolation forest](https://en.wikipedia.org/wiki/Isolation_forest) (Multivariate)
Detects anomalies using binary trees. The algorithm has a linear time complexity and a low memory requirement, which works well with high-volume data. It can be used on both univatiate and multivariate data, but it is more effective in multivariate case.

**Important**: Be aware of [the curse of dimensionality](https://en.wikipedia.org/wiki/Curse_of_dimensionality). Don't use single multivariate model if you expect your queries to return many time series of less datapoints that the number of metrics. In such case it is hard for a model to learn meaningful dependencies from too sparse data hypercube.

Here we use Isolation Forest implementation from `scikit-learn` [library](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). All parameters from this library can be passed to the model.

*Parameters specific for vmanomaly*:

* `class` (string) - model class name `"model.isolation_forest.IsolationForestMultivariateModel"`

* `contamination` (float or string, optional) - The amount of contamination of the data set, i.e. the proportion of outliers in the data set. Used when fitting to define the threshold on the scores of the samples. Default value - "auto". Should be either `"auto"` or be in the range (0.0, 0.5].

* `args` (dict, optional) - Inner model args (key-value pairs). See accepted params in [model documentation](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html). Defaults to empty (not provided). Example:  {"random_state": 42, "n_estimators": 100}

*Config Example*


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


Resulting metrics of the model are described [here](#vmanomaly-output).

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

Each model exposes [several healthchecks metrics](/anomaly-detection/components/monitoring.html#models-behaviour-metrics) to its `health_path` endpoint:


## Custom Model Guide

Apart from vmanomaly predefined models, users can create their own custom models for anomaly detection.

Here in this guide, we will
- Make a file containing our custom model definition
- Define VictoriaMetrics Anomaly Detection config file to use our custom model
- Run service

**Note**: The file containing the model should be written in [Python language](https://www.python.org/) (3.11+)

### 1. Custom model

We'll create `custom_model.py` file with `CustomModel` class that will inherit from vmanomaly `Model` base class.
In the `CustomModel` class there should be three required methods - `__init__`, `fit` and `infer`:
* `__init__` method should initiate parameters for the model.

  **Note**: if your model relies on configs that have `arg` [key-value pair argument](./models.md#section-overview), do not forget to use Python's `**kwargs` in method's signature and to explicitly call
  ```python 
  super().__init__(**kwargs)
  ``` 
  to initialize the base class each model derives from
* `fit` method should contain the model training process.
* `infer` should return Pandas.DataFrame object with model's inferences.

For the sake of simplicity, the model in this example will return one of two values of `anomaly_score` - 0 or 1 depending on input parameter `percentage`.


```python
import numpy as np
import pandas as pd
import scipy.stats as st
import logging

from model.model import Model
logger = logging.getLogger(__name__)


class CustomModel(Model):
    """
    Custom model implementation.
    """

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

```



### 2. Configuration file

Next, we need to create `config.yaml` file with VM Anomaly Detection configuration and model input parameters.
In the config file `model` section we need to put our model class `model.custom.CustomModel` and all parameters used in `__init__` method.
You can find out more about configuration parameters in vmanomaly docs.


```yaml
scheduler:
  infer_every: "1m"
  fit_every: "1m"
  fit_window: "1d"

model:
  # note: every custom model should implement this exact path, specified in `class` field
  class: "model.model.CustomModel"
  # custom model params are defined here
  percentage: 0.9

reader:
  datasource_url: "http://localhost:8428/"
  queries:
    ingestion_rate: 'sum(rate(vm_rows_inserted_total)) by (type)'
    churn_rate: 'sum(rate(vm_new_timeseries_created_total[5m]))'

writer:
  datasource_url: "http://localhost:8428/"
  metric_format:
    __name__: "custom_$VAR"
    for: "$QUERY_KEY"
    model: "custom"
    run: "test-format"

monitoring:
  # /metrics server.
  pull:
    port: 8080
  push:
    url: "http://localhost:8428/"
    extra_labels:
      job: "vmanomaly-develop"
      config: "custom.yaml"
```


### 3. Running custom model
Let's pull the docker image for vmanomaly:


```sh 
docker pull us-docker.pkg.dev/victoriametrics-test/public/vmanomaly-trial:latest
```


Now we can run the docker container putting as volumes both config and model file:

**Note**: place the model file to `/model/custom.py` path when copying

```sh
docker run -it \
--net [YOUR_NETWORK] \
-v [YOUR_LICENSE_FILE_PATH]:/license.txt \
-v $(PWD)/custom_model.py:/vmanomaly/src/model/custom.py \
-v $(PWD)/custom.yaml:/config.yaml \
us-docker.pkg.dev/victoriametrics-test/public/vmanomaly-trial:latest /config.yaml \
--license-file=/license.txt
```


Please find more detailed instructions (license, etc.) [here](/anomaly-detection/overview.html#run-vmanomaly-docker-container)


### Output
As the result, this model will return metric with labels, configured previously in `config.yaml`.
In this particular example, 2 metrics will be produced. Also, there will be added other metrics from input query result.

```
{__name__="custom_anomaly_score", for="ingestion_rate", model="custom", run="test-format"}

{__name__="custom_anomaly_score", for="churn_rate", model="custom", run="test-format"}
```
