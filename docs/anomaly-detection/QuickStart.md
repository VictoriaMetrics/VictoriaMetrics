---
weight: 1
title: Quick Start
menu:
  docs:
    parent: "anomaly-detection"
    identifier: "vmanomaly-quick-start"
    weight: 1
    title: Quick Start
aliases:
- /anomaly-detection/QuickStart.html
---
For a broader overview please visit the [navigation page](https://docs.victoriametrics.com/anomaly-detection/).

## How to install and run vmanomaly

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

The following options are available:

- [To run Docker image](#docker)
- [To run in Kubernetes with Helm charts](#kubernetes-with-helm-charts)

> **Note**: There is a mode {{% available_from "v1.13.0" anomaly %}} to keep anomaly detection models on host filesystem after `fit` stage (instead of keeping them in-memory by default); This may lead to **noticeable reduction of RAM used** on bigger setups. Similar optimization {{% available_from "v1.16.0" anomaly %}} can be set for data read from VictoriaMetrics TSDB. See instructions [here](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).

### Command-line arguments

The `vmanomaly` service supports several command-line arguments to configure its behavior, including options for licensing, logging levels, and more. These arguments can be passed when starting the service via Docker or any other setup. Below is the list of available options:

> **Note**: `vmanomaly` support {{% available_from "v1.18.5" anomaly %}} running on config *directories*, see the `config` positional arg description in help message below.

```shellhelp
usage: vmanomaly.py [-h] [--license STRING | --licenseFile PATH] [--license.forceOffline] [--loggerLevel {INFO,DEBUG,ERROR,WARNING,FATAL}] [--watch] config [config ...]

VictoriaMetrics Anomaly Detection Service

positional arguments:
  config                YAML config file(s) or directories containing YAML files. Multiple files will recursively merge each other values so multiple configs can be combined. If a directory
                        is provided, all `.yaml` files inside will be merged, without recursion. Default: vmanomaly.yaml is expected in the current directory.

options:
  -h                    show this help message and exit
  --license STRING      License key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/trial/ to obtain a trial license.
  --licenseFile PATH    Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/trial/ to obtain a trial license.
  --license.forceOffline 
                        Whether to force offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The
                        issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification.
  --loggerLevel {INFO,DEBUG,ERROR,WARNING,FATAL}
                        Minimum level to log. Possible values: DEBUG, INFO, WARNING, ERROR, FATAL.
  --watch               [DEPRECATED SINCE v1.11.0] Watch config files for changes. This option is no longer supported and will be ignored.
```

You can specify these options when running `vmanomaly` to fine-tune logging levels or handle licensing configurations, as per your requirements.

### Licensing

The license key can be passed via the following command-line flags: `--license`, `--licenseFile`, `--license.forceOffline`

In order to make it easier to monitor the license expiration date, the following metrics are exposed(see
[Monitoring](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/) section for details on how to scrape them):

```promtextmetric
# HELP vm_license_expires_at When the license expires as a Unix timestamp in seconds
# TYPE vm_license_expires_at gauge
vm_license_expires_at 1.6963776e+09
# HELP vm_license_expires_in_seconds Amount of seconds until the license expires
# TYPE vm_license_expires_in_seconds gauge
vm_license_expires_in_seconds 4.886608e+06
```

Example alerts for [vmalert](https://docs.victoriametrics.com/vmalert/):

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

### Docker

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/). <br><br>
> Due to the upcoming [DockerHub pull limits](https://docs.docker.com/docker-hub/usage/pulls), an additional image registry, **Quay.io**, has been introduced for VictoriaMetrics images, including [`vmanomaly`](https://quay.io/repository/victoriametrics/vmanomaly). If you encounter pull rate limits, switch from:  
> ```
> docker pull victoriametrics/vmanomaly:vX.Y.Z
> ```
> to:  
> ```
> docker pull quay.io/victoriametrics/vmanomaly:vX.Y.Z
> ```


Below are the steps to get `vmanomaly` up and running inside a Docker container:

1. Pull Docker image:

```sh
docker pull victoriametrics/vmanomaly:v1.20.1
```

2. (Optional step) tag the `vmanomaly` Docker image:

```sh
docker image tag victoriametrics/vmanomaly:v1.20.1 vmanomaly
```

3. Start the `vmanomaly` Docker container with a *license file*, use the command below.
**Make sure to replace `YOUR_LICENSE_FILE_PATH`, and `YOUR_CONFIG_FILE_PATH` with your specific details**:

```sh
export YOUR_LICENSE_FILE_PATH=path/to/license/file
export YOUR_CONFIG_FILE_PATH=path/to/config/file
docker run -it -v $YOUR_LICENSE_FILE_PATH:/license \
               -v $YOUR_CONFIG_FILE_PATH:/config.yml \
               vmanomaly /config.yml \
               --licenseFile=/license \
               --loggerLevel=INFO
```

In case you found `PermissionError: [Errno 13] Permission denied:` in `vmanomaly` logs, set user/user group to 1000 in the run command above / in a docker-compose file:

```sh
export YOUR_LICENSE_FILE_PATH=path/to/license/file
export YOUR_CONFIG_FILE_PATH=path/to/config/file
docker run -it --user 1000:1000 \
               -v $YOUR_LICENSE_FILE_PATH:/license \
               -v $YOUR_CONFIG_FILE_PATH:/config.yml \
               vmanomaly /config.yml \
               --licenseFile=/license \
               --loggerLevel=INFO
```

```yaml
# docker-compose file
services:
  # ...
  vmanomaly:
    image: victoriametrics/vmanomaly:v1.21.0
    volumes:
        $YOUR_LICENSE_FILE_PATH:/license
        $YOUR_CONFIG_FILE_PATH:/config.yml
    command:
      - "/config.yml"
      - "--licenseFile=/license"
      - "--loggerLevel=INFO"
    # ...
```

For a complete docker-compose example please refer to [our alerting guide](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/), chapter [docker-compose](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/#docker-compose)



See also:

- Verify the license online OR offline. See the details [here](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing).
- [How to configure `vmanomaly`](#how-to-configure-vmanomaly)

### Kubernetes with Helm charts

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

> With the forthcoming [DockerHub pull limits](https://docs.docker.com/docker-hub/usage/pulls) additional image registry was introduced (quay.io) for VictoriaMetric images, [vmanomaly images in particular](https://quay.io/repository/victoriametrics/vmanomaly).
If hitting pull limits, try switching your `docker pull quay.io/victoriametrics/vmanomaly:vX.Y.Z` to `docker pull quay.io/victoriametrics/vmanomaly:vX.Y.Z`

You can run `vmanomaly` in Kubernetes environment
with [these Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/README.md).

## How to configure vmanomaly
To run `vmanomaly` you need to set up configuration file in `yaml` format.

Here is an example of config file that will run [Facebook Prophet](https://facebook.github.io/prophet/) model, that will be retrained every 2 hours on 14 days of previous data. It will generate [inference metrics](https://docs.victoriametrics.com/anomaly-detection/components/models#vmanomaly-output) (including `anomaly_score`) every 1 minute.


```yaml
schedulers:
  1d_1m:
    # https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler
    class: 'periodic'
    infer_every: '1m'
    fit_every: '1d'
    fit_window: '2w'

models:
  # https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet
  prophet_model:
    class: 'prophet'
    provide_series: ['anomaly_score', 'yhat', 'yhat_lower', 'yhat_upper']  # for debugging
    tz_aware: True
    tz_use_cyclical_encoding: True
    tz_seasonalities: # intra-day + intra-week seasonality
      - name: 'hod'  # intra-day seasonality, hour of the day
        fourier_order: 4  # keep it 3-8 based on intraday pattern complexity
        prior_scale: 10
      - name: 'dow'  # intra-week seasonality, time of the week
        fourier_order: 2  # keep it 2-4, as dependencies are learned separately for each weekday
    # inner model args (key-value pairs) accepted by
    # https://facebook.github.io/prophet/docs/quick_start#python-api
    args:
      interval_width: 0.98  # see https://facebook.github.io/prophet/docs/uncertainty_intervals

reader:
  # https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader
  datasource_url: "http://victoriametrics:8428/" # [YOUR_DATASOURCE_URL]
  sampling_period: "1m"
  queries: 
    # define your queries with MetricsQL - https://docs.victoriametrics.com/metricsql/
    cache: "sum(rate(vm_cache_entries))"

writer:
  # https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer
  datasource_url:  "http://victoriametrics:8428/" # [YOUR_DATASOURCE_URL]
```

### Recommended steps

**Schedulers**:
- Configure the **inference frequency** in the [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) section of the configuration file.  
- Ensure that `infer_every` aligns with your **minimum required alerting frequency**.  
  - For example, if receiving **alerts every 15 minutes** is sufficient (when `anomaly_score > 1`), set `infer_every` to match `reader.sampling_period` or override it per query via `reader.queries.query_xxx.step` for an optimal setup.  

**Reader**:
- Setup the datasource to read data from in the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/) section. Include tenant ID if using a [cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/) (`multitenant` value {{% available_from "v1.16.2" anomaly %}} can be also used here).
- Define queries for input data using [MetricsQL](https://docs.victoriametrics.com/metricsql/) under `reader.queries` section. Note, it's possible to override reader-level arguments at query level for increased flexibility, e.g. specifying per-query timezone, data frequency, data range, etc.

**Writer**:
- Specify where and how to store anomaly detection metrics in the [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/) section.
- Include tenant ID if using a [cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/) for writing the results.
- Adding `for` label to `metric_format` argument is recommended for smoother visual experience in the [anomaly score dashboard](https://docs.victoriametrics.com/anomaly-detection/presets/#default). Please refer to `metric_format` argument description [here](https://docs.victoriametrics.com/anomaly-detection/components/writer/?highlight=metric_format#config-parameters).

**Models**:
- Configure built-in models parameters according to your needs in the [models](https://docs.victoriametrics.com/anomaly-detection/components/models/) section. Where possible, incorporate [domain knowledge](https://docs.victoriametrics.com/anomaly-detection/faq/#incorporating-domain-knowledge) for optimal results.
- (Optional) Develop or integrate your [custom models](https://docs.victoriametrics.com/anomaly-detection/components/models/#custom-model-guide) with `vmanomaly`.
- Adding `y` to `provide_series` arg values is recommended for smoother visual experience in the [anomaly score dashboard](https://docs.victoriametrics.com/anomaly-detection/presets/#default). Also, other `vmanomaly` [output](https://docs.victoriametrics.com/anomaly-detection/components/models#vmanomaly-output) can be used in `provide_series`. <br>**Note:** Only [univariate models](https://docs.victoriametrics.com/anomaly-detection/components/models/#univariate-models) support the generation of such output.

## Check also

Here are the links for further deep dive into Anomaly Detection in general and `vmanomaly` in particular:

- [High Availability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#high-availability)
- [Horizontal Scalability](https://docs.victoriametrics.com/anomaly-detection/scaling-vmanomaly/#horizontal-scalability)
- [Guide: Anomaly Detection and Alerting Setup](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/)
- [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/)
- [CHANGELOG](https://docs.victoriametrics.com/anomaly-detection/changelog/)
- [Anomaly Detection Blog](https://victoriametrics.com/tags/anomaly-detection/)
