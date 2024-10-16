---
weight: 1
title: VictoriaMetrics Anomaly Detection Quick Start
menu:
  docs:
    parent: "anomaly-detection"
    weight: 1
    title: Quick Start
aliases:
- /anomaly-detection/QuickStart.html
---
For service introduction visit [README](https://docs.victoriametrics.com/anomaly-detection/) page
and [Overview](https://docs.victoriametrics.com/anomaly-detection/overview/) of how `vmanomaly` works.

## How to install and run vmanomaly

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

The following options are available:

- [To run Docker image](#docker)
- [To run in Kubernetes with Helm charts](#kubernetes-with-helm-charts)

> **Note**: Starting from [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130) there is a mode to keep anomaly detection models on host filesystem after `fit` stage (instead of keeping them in-memory by default); This may lead to **noticeable reduction of RAM used** on bigger setups. See instructions [here](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).

> **Note**: Starting from [v1.16.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1160), a similar optimization is available for data read from VictoriaMetrics TSDB. See instructions [here](https://docs.victoriametrics.com/anomaly-detection/faq/#on-disk-mode).

### Docker

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

Below are the steps to get `vmanomaly` up and running inside a Docker container:

1. Pull Docker image:

```sh
docker pull victoriametrics/vmanomaly:v1.16.3
```

2. (Optional step) tag the `vmanomaly` Docker image:

```sh
docker image tag victoriametrics/vmanomaly:v1.16.3 vmanomaly
```

3. Start the `vmanomaly` Docker container with a *license file*, use the command below.
**Make sure to replace `YOUR_LICENSE_FILE_PATH`, and `YOUR_CONFIG_FILE_PATH` with your specific details**:

```sh
export YOUR_LICENSE_FILE_PATH=path/to/license/file
export YOUR_CONFIG_FILE_PATH=path/to/config/file
docker run -it -v $YOUR_LICENSE_FILE_PATH:/license \
               -v $YOUR_CONFIG_FILE_PATH:/config.yml \
               vmanomaly /config.yml \
               --licenseFile=/license
```

In case you found `PermissionError: [Errno 13] Permission denied:` in `vmanomaly` logs, set user/user group to 1000 in the run command above / in a docker-compose file:

```sh
export YOUR_LICENSE_FILE_PATH=path/to/license/file
export YOUR_CONFIG_FILE_PATH=path/to/config/file
docker run -it --user 1000:1000 \
               -v $YOUR_LICENSE_FILE_PATH:/license \
               -v $YOUR_CONFIG_FILE_PATH:/config.yml \
               vmanomaly /config.yml \
               --licenseFile=/license
```

```yaml
# docker-compose file
services:
  # ...
  vmanomaly:
    image: victoriametrics/vmanomaly:v1.16.3
    volumes:
        $YOUR_LICENSE_FILE_PATH:/license
        $YOUR_CONFIG_FILE_PATH:/config.yml
    command:
      - "/config.yml"
      - "--licenseFile=/license"
    # ...
```

For a complete docker-compose example please refer to [our alerting guide](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/), chapter [docker-compose](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/#docker-compose)



See also:

- Verify the license online OR offline. See the details [here](https://docs.victoriametrics.com/anomaly-detection/overview/#licensing).
- [How to configure `vmanomaly`](#how-to-configure-vmanomaly)

### Kubernetes with Helm charts

> To run `vmanomaly`, you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/).

You can run `vmanomaly` in Kubernetes environment
with [these Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/README.md).


## How to configure vmanomaly
To run `vmanomaly` you need to set up configuration file in `yaml` format.

Here is an example of config file that will run [Facebook Prophet](https://facebook.github.io/prophet/) model, that will be retrained every 2 hours on 14 days of previous data. It will generate inference (including `anomaly_score` metric) every 1 minute.


```yaml
schedulers:
  2h_1m:
    # https://docs.victoriametrics.com/anomaly-detection/components/scheduler/#periodic-scheduler
    class: 'periodic'
    infer_every: '1m'
    fit_every: '2h'
    fit_window: '2w'

models:
  # https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet
  prophet_model:
    class: "prophet"  # or "model.prophet.ProphetModel" until v1.13.0
    args:
      interval_width: 0.98

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


Next steps:
- Define how often to run and make inferences in the [scheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/) section of a config file.
- Setup the datasource to read data from in the [reader](https://docs.victoriametrics.com/anomaly-detection/components/reader/) section.
- Specify where and how to store anomaly detection metrics in the [writer](https://docs.victoriametrics.com/anomaly-detection/components/writer/) section.
- Configure built-in models parameters according to your needs in the [models](https://docs.victoriametrics.com/anomaly-detection/components/models/) section.
- Integrate your [custom models](https://docs.victoriametrics.com/anomaly-detection/components/models/#custom-model-guide) with `vmanomaly`.
- Define queries for input data using [MetricsQL](https://docs.victoriametrics.com/metricsql/).


## Check also

Here are other materials that you might find useful:

- [Guide: Anomaly Detection and Alerting Setup](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/)
- [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/)
- [Changelog](https://docs.victoriametrics.com/anomaly-detection/changelog/)
- [Anomaly Detection Blog](https://victoriametrics.com/blog/tags/anomaly-detection/)
