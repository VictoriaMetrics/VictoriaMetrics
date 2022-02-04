---
sort: 20
---

# vmanomaly

**_vmanomaly is a part of [enterprise package](https://victoriametrics.com/products/enterprise/).
Please [contact us](https://victoriametrics.com/contact-us/) to find out more_**

## About

**VictoriaMetrics Anomaly Detection** is a service that continuously scans Victoria Metrics time
series and detects unexpected changes within data patterns in real time. It does so by utilizing
user-configurable machine learning models.

It periodically queries user-specified metrics, computes an “anomaly score” for them, based on how
well they fit a predicted distribution, taking into account periodical data patterns with trends,
and pushes back the computed “anomaly score” to Victoria Metrics. Then, users can enable alerting
rules based on the “anomaly score”.

Compared to classical alerting rules, anomaly detection is more “hands off” i.e. it allows users to
avoid setting up manual alerting rules set up and catch anomalies that were not expected to happen.
In other words, by setting up alerting rules, a user must know what to look for, ahead of time,
while anomaly detection looks for any deviations from past behavior.

In addition to that, setting up alerting rules manually has been proven to be tedious and
error-prone, while anomaly detection can be easier to set up, and use the same model for different
metrics.

## How?

Victoria Metrics Anomaly Detection service (**vmanomaly**) allows you to apply several built-in
anomaly detection algorithms. You can also plug in your own detection models, code doesn’t make any
distinction between built-in models or external ones.

All the service parameters (model, schedule, input-output) are defined in a config file.

Single config file supports only one model, but it’s totally OK to run multiple **vmanomaly**
processes in parallel, each using its own config.

## Models

Currently, vmanomaly ships with a few common models:

1. **ZScore**

   _(useful for testing)_

   Simplistic model, that detects outliers as all the points that lie farther than a certain amount
   from time series mean (straight line). Keeps only two model parameters internally:
   `mean` and `std` (standard deviation).

2. **Prophet**

   _(simplest in configuration, recommended for getting starting)_

   Uses Facebook Prophet for forecasting. Anomaly score is computed of how close the actual time
   series values follow the forecasted values (yhat), and whether it’s within forecasted bounds
   (_yhat_lower_, _yhat_upper_). Anomaly score reaches 1.0 if the actual data values equal to
   _yhat_lower_ or _yhat_upper_. Anomaly score is above 1.0 if the actual data values are outside
   the _yhat_lower_/_yhat_upper_ bounds.

   See Prophet documentation: https://facebook.github.io/prophet/

3. **Holt-Winters**

   Very popular forecasting algorithm. See [statsmodels.org documentation](
   https://www.statsmodels.org/stable/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html)
   for Holt-Winters exponential smoothing.

4. **Seasonal-Trend Decomposition**

   Extracts three components: season, trend and residual, that can be plotted individually for
   easier debugging. Uses LOESS (locally estimated scatterplot smoothing).
   See [statsmodels.org documentation](https://www.statsmodels.org/dev/examples/notebooks/generated/stl_decomposition.html)
   for LOESS STD.

5. **ARIMA**

   Commonly used forecasting model. See [statsmodels.org documentation](https://www.statsmodels.
   org/stable/generated/statsmodels.tsa.arima.model.ARIMA.html) for ARIMA.

6. **Rolling Quantile**

   Simple moving window of quantiles.


### Examples
For example here’s how Prophet predictions could look like on example of real data  
(Prophet auto-detected seasonality interval):
[prophet](prophet.png)

And here’s how Holt-Winters predictions real-world data could look like (seasonality manually 
 set to 1 week). Notice that it predicts anomalies in 
different places than Prophet, because the model noticed there are usually spikes on Friday 
morning, so it accounted for that:
[hw](hw.png)

## Process
Upon starting, vmanomaly queries the initial range of data, and trains its model (“fit” by convention).

Then, reads new data from VictoriaMetrics, according to schedule, and invokes its model to compute 
“anomaly score” for each data point. The anomaly score ranges from 0 to positive infinity. 
Values less than 1.0 are considered “not anomaly”, values greater or equal than 1.0 are 
considered “anomalous”, with greater values corresponding to larger anomaly.
Then, VMAnomaly pushes the metric to vminsert (under user-configured metric name, 
optionally preserving labels).
The writer is also pluggable, users can supply their own writers.


## Usage
Script accepts only one parameter -- config file path
`python3 -m vmanomaly config_zscore.yaml`

It is also possible to split up config to multiple files, just list them all in command line 
`python3 -m vmanomaly model_prophet.yaml io_csv.yaml scheduler_oneoff.yaml`
