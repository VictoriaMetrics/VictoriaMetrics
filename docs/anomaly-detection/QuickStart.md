---
sort: 1
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

# VictoriaMetrics Anomaly Detection Quick Start

It is recommended to read [README](https://docs.victoriametrics.com/anomaly-detection/)
and [Overview](https://docs.victoriametrics.com/anomaly-detection/overview.html)
before you start working with `vmanomaly`.

## How to install and run `vmanomaly`

> To run `vmanomaly` you need to have VictoriaMetrics Enterprise license. You can get a trial license key [**here**](https://victoriametrics.com/products/enterprise/trial/index.html).

The following options are available:

- [To run Docker image](#docker-image)
- [To run in Kubernetes with Helm charts](#helm-charts)


### Docker image

The simplest and quickest method to deploy `vmanomaly` is through Docker. Below are the steps to get `vmanomaly` up and running inside a Docker container.

First, you can (optionally) tag the `vmanomaly` Docker image for convenience using the following command:

```sh
docker image tag victoriametrics/vmanomaly:latest vmanomaly
```

Second, start the `vmanomaly` Docker container with a *license file*, use the command below.
Make sure to replace `YOUR_LICENSE_FILE_PATH`, and `YOUR_CONFIG_FILE_PATH` with your specific details:

```sh
export YOUR_LICENSE_FILE_PATH=path/to/license/file
export YOUR_CONFIG_FILE_PATH=path/to/config/file
docker run -it -v $YOUR_LICENSE_FILE_PATH:/license \
               -v $YOUR_CONFIG_FILE_PATH:/config.yml \
               vmanomaly /config.yml \
               --license-file=/license
```

See also:

- You can verify licence online and offline. See the details [here](https://docs.victoriametrics.com/anomaly-detection/overview/#licensing).
- [How to configure `vmanomaly`](#how-to-configure-vmanomaly)

### Kubernetes with Helm charts

You can run `vmanomaly` in Kubernetes environment
with [these Helm charts](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-anomaly/README.md).


## How to configure vmanomaly
To run `vmanomaly` you need to set up configuration file in `yaml` format.

Here is an example of config file that will run FB Prophet model, that will be retrained every 2 hours on 14 days of previous data. It will generate inference (including `anomaly_score` metric) every 1 minute.


```yaml
scheduler:
  infer_every: "1m"
  fit_every: "2h"
  fit_window: "14d"

model:
  class: "model.prophet.ProphetModel"
  args:
    interval_width: 0.98

reader:
  datasource_url: "http://victoriametrics:8428/" # [YOUR_DATASOURCE_URL]
  sampling_period: "1m"
  queries: 
    # define your queries with MetricsQL - https://docs.victoriametrics.com/metricsql/
    cache: "sum(rate(vm_cache_entries))"

writer:
  datasource_url:  "http://victoriametrics:8428/" # [YOUR_DATASOURCE_URL]
```


See also:

- You can configure `vmanomaly` schedule, model, reading and writing parameters according to your needs. See the details [here](https://docs.victoriametrics.com/anomaly-detection/components/)
- Built-in [models and their parameters](https://docs.victoriametrics.com/anomaly-detection/components/models/)
- To define queries for input data use [MetricsQL](https://docs.victoriametrics.com/metricsql/)


## Check also

Here are other materials that you might find useful:

- [Guide: Anomaly Detection and Alerting Setup](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/)
- [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/)
- [Changelog](https://docs.victoriametrics.com/anomaly-detection/changelog/)
- [Anomaly Detection Blog](https://victoriametrics.com/blog/tags/anomaly-detection/)