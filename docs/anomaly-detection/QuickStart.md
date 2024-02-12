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

# `vmanomaly` Quick Start

It is recommended to read [README](https://docs.victoriametrics.com//vmanomaly.html/)
and [Overview](https://docs.victoriametrics.com/anomaly-detection/overview.html)
before you start working with `vmanomaly`.

## How to install and run `vmanomaly`

>`vmanomaly` is a part of VictoriaMetrics Enterprise version. You can get a license key [**here**](https://victoriametrics.com/products/enterprise/trial/index.html).

There are the following options exist:

- [To run Docker image](#docker-image)
- [To run in Kubernetes with Helm charts](#helm-charts)


### Docker image

You can run `vmanomaly` in a Docker container. It is the easiest way to start using `vmanomaly`.
Here is the command to run `vmanomaly` in a Docker container:

You can put a tag on it for your convinience:

```sh
docker image tag victoriametrics/vmanomaly:latest vmanomaly
```
Here is an example of how to run *vmanomaly* docker container with *license file*. 

```sh
docker run -it --net [YOUR_NETWORK] \
               -v [YOUR_LICENSE_FILE_PATH]:/license \
               -v [YOUR_CONFIG_FILE_PATH]:/config.yml \
               vmanomaly /config.yml \
               --license-file=/license
```

See also:

- [All license parameters](https://docs.victoriametrics.com/anomaly-detection/overview/#licensing).
- [How to configure `vmanomaly`](#how-to-configure-vmanomaly)

### Helm charts

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
  queries:
    cache: "sum(rate(vm_cache_entries))"

writer:
  datasource_url:  "http://victoriametrics:8428/" # [YOUR_DATASOURCE_URL]
```


See also:

- [Config components](https://docs.victoriametrics.com/anomaly-detection/components/)
- [Models](https://docs.victoriametrics.com/anomaly-detection/components/models/)
- [MetricsQl](https://docs.victoriametrics.com/metricsql/)


## Other assets

Here are other materials that you might find useful:

- [Guide: Anomaly Detection and Alerting Setup](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/)
- [FAQ](https://docs.victoriametrics.com/anomaly-detection/faq/)
- [Changelog](https://docs.victoriametrics.com/anomaly-detection/changelog/)
- [Anomaly Detection Blog](https://victoriametrics.com/blog/tags/anomaly-detection/)