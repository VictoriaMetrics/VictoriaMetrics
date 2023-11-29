---
sort: 11
weight: 11
title: vmanomaly
menu:
  docs:
    parent: 'victoriametrics'
    weight: 11
aliases:
- /vmanomaly.html
---

# vmanomaly

**_vmanomaly is a part of [enterprise package](https://docs.victoriametrics.com/enterprise.html). You need to request a [free trial license](https://victoriametrics.com/products/enterprise/trial/) for evaluation.
Please [contact us](https://victoriametrics.com/contact-us/) to find out more._**

## About

**VictoriaMetrics Anomaly Detection** is a service that continuously scans VictoriaMetrics time
series and detects unexpected changes within data patterns in real-time. It does so by utilizing
user-configurable machine learning models.

It periodically queries user-specified metrics, computes an “anomaly score” for them, based on how
well they fit a predicted distribution, taking into account periodical data patterns with trends,
and pushes back the computed “anomaly score” to VictoriaMetrics. Then, users can enable alerting
rules based on the “anomaly score”.

Compared to classical alerting rules, anomaly detection is more “hands-off” i.e. it allows users to
avoid setting up manual alerting rules set up and catching anomalies that were not expected to happen.
In other words, by setting up alerting rules, a user must know what to look for, ahead of time,
while anomaly detection looks for any deviations from past behavior.

In addition to that, setting up alerting rules manually has been proven to be tedious and
error-prone, while anomaly detection can be easier to set up, and use the same model for different
metrics.

## How?

VictoriaMetrics Anomaly Detection service (**vmanomaly**) allows you to apply several built-in
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
   from time-series mean (straight line). Keeps only two model parameters internally:
   `mean` and `std` (standard deviation).

1. **Prophet**

   _(simplest in configuration, recommended for getting starting)_

   Uses Facebook Prophet for forecasting. The _anomaly score_ is computed of how close the actual time
   series values follow the forecasted values (_yhat_), and whether it’s within forecasted bounds
   (_yhat_lower_, _yhat_upper_). The _anomaly score_ reaches 1.0 if the actual data values 
   are equal to
   _yhat_lower_ or _yhat_upper_. The _anomaly score_ is above 1.0 if the actual data values are 
   outside
   the _yhat_lower_/_yhat_upper_ bounds.

   See [Prophet documentation](https://facebook.github.io/prophet/)

1. **Holt-Winters**

   Very popular forecasting algorithm. See [statsmodels.org documentation](
   https://www.statsmodels.org/stable/generated/statsmodels.tsa.holtwinters.ExponentialSmoothing.html)
   for Holt-Winters exponential smoothing.

1. **Seasonal-Trend Decomposition**

   Extracts three components: season, trend, and residual, that can be plotted individually for
   easier debugging. Uses LOESS (locally estimated scatterplot smoothing).
   See [statsmodels.org documentation](https://www.statsmodels.org/dev/examples/notebooks/generated/stl_decomposition.html)
   for LOESS STD.

1. **ARIMA**

   Commonly used forecasting model. See [statsmodels.org documentation](https://www.statsmodels.org/stable/generated/statsmodels.tsa.arima.model.ARIMA.html) for ARIMA.

1. **Rolling Quantile**

   A simple moving window of quantiles. Easy to use, easy to understand, but not as powerful as 
   other models.

1. **Isolation Forest**

   Detects anomalies using binary trees. It works for both univariate and multivariate data. Be aware of [the curse of dimensionality](https://en.wikipedia.org/wiki/Curse_of_dimensionality) in the case of multivariate data - we advise against using a single model when handling multiple time series *if the number of these series significantly exceeds their average length (# of data points)*.
   
   The algorithm has a linear time complexity and a low memory requirement, which works well with high-volume data. See [scikit-learn.org documentation](https://scikit-learn.org/stable/modules/generated/sklearn.ensemble.IsolationForest.html) for Isolation Forest.


### Examples
For example, here’s how Prophet predictions could look like on a real-data example  
(Prophet auto-detected seasonality interval):

<img alt="propher-example" src="vmanomaly-prophet-example.webp">

And here’s what Holt-Winters predictions real-world data could look like (seasonality manually 
 set to 1 week). Notice that it predicts anomalies in 
different places than Prophet because the model noticed there are usually spikes on Friday 
morning, so it accounted for that:

<img alt="holtwinters-example" src="vmanomaly-holtwinters-example.webp">

## Process
Upon starting, vmanomaly queries the initial range of data, and trains its model (“fit” by convention).

Then, reads new data from VictoriaMetrics, according to schedule, and invokes its model to compute 
“anomaly score” for each data point. The anomaly score ranges from 0 to positive infinity. 
Values less than 1.0 are considered “not an anomaly”, values greater or equal than 1.0 are 
considered “anomalous”, with greater values corresponding to larger anomaly.
Then, vmanomaly pushes the metric to vminsert (under the user-configured metric name, 
optionally preserving labels).

 
## Usage
The vmanomaly accepts only one parameter -- config file path:

```sh
python3 vmanomaly.py config_zscore.yaml
```
or
```sh
python3 -m vmanomaly config_zscore.yaml
```

It is also possible to split up config into multiple files, just list them all in the command line:

```sh
python3 -m vmanomaly model_prophet.yaml io_csv.yaml scheduler_oneoff.yaml
```

### Monitoring

vmanomaly can be monitored by using push or pull approach.
It can push metrics to VictoriaMetrics or expose metrics in Prometheus exposition format.

#### Push approach

vmanomaly can push metrics to VictoriaMetrics single-node or cluster version.
In order to enable push approach, specify `push` section in config file:

```yaml
monitoring:
   push:
      url: "http://victoriametrics:8428/"
      extra_labels:
         job: "vmanomaly-push"
```

#### Pull approach

vmanomaly can export internal metrics in Prometheus exposition format at `/metrics` page.
These metrics can be scraped via [vmagent](https://docs.victoriametrics.com/vmagent.html) or Prometheus.

In order to enable pull approach, specify `pull` section in config file:

```yaml
monitoring:
   pull:
      enable: true
      port: 8080
```

This will expose metrics at `http://0.0.0.0:8080/metrics` page.

### Licensing

Starting from v1.5.0 vmanomaly requires a license key to run. You can obtain a trial license
key [here](https://victoriametrics.com/products/enterprise/trial/).

The license key can be passed via the following command-line flags:
```
  --license LICENSE     See https://victoriametrics.com/products/enterprise/
                        for trial license
  --license-file LICENSE_FILE
                        See https://victoriametrics.com/products/enterprise/
                        for trial license
  --license-verify-offline {true,false}
                        Force offline verification of license code. License is
                        verified online by default. This flag runs license
                        verification offline.
```

Usage example:
```
python3 -m vmanomaly --license-file /path/to/license_file.yaml config.yaml
```

In order to make it easier to monitor the license expiration date, the following metrics are exposed(see
[Monitoring](#monitoring) section for details on how to scrape them):

```
# HELP vm_license_expires_at When the license expires as a Unix timestamp in seconds
# TYPE vm_license_expires_at gauge
vm_license_expires_at 1.6963776e+09
# HELP vm_license_expires_in_seconds Amount of seconds until the license expires
# TYPE vm_license_expires_in_seconds gauge
vm_license_expires_in_seconds 4.886608e+06
```

Example alerts for [vmalert](https://docs.victoriametrics.com/vmalert.html):
{% raw %}
```yaml
groups:
  - name: vm-license
    # note the `job` label and update accordingly to your setup
    rules:
      - alert: LicenseExpiresInLessThan30Days
        expr: vm_license_expires_in_seconds < 30 * 24 * 3600
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.job }} instance {{ $labels.instance }} license expires in less than 30 days"
          description: "{{ $labels.instance }} of job {{ $labels.job }} license expires in {{ $value | humanizeDuration }}. 
            Please make sure to update the license before it expires."

      - alert: LicenseExpiresInLessThan7Days
        expr: vm_license_expires_in_seconds < 7 * 24 * 3600
        labels:
          severity: critical
        annotations:
          summary: "{{ $labels.job }} instance {{ $labels.instance }} license expires in less than 7 days"
          description: "{{ $labels.instance }} of job {{ $labels.job }} license expires in {{ $value | humanizeDuration }}. 
            Please make sure to update the license before it expires."
```
{% endraw %}
