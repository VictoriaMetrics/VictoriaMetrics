# Docker Compose file for "vmanomaly integration" guide

Please read the "vmanomaly integration" guide first - [https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert.html](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert.html)

To make this Docker compose file work, you MUST replace the content of [vmanomaly_license](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker/vmanomaly/vmanomaly-integration/vmanomaly_license) with valid license.

You can issue the [trial license here](https://victoriametrics.com/products/enterprise/trial/)


## How to run 

1. Replace content of `vmanomaly_license` with your license
1. Run

```sh 
docker compose up -d  
```
1. Open Grafana on  `http://127.0.0.1:3000/`
```sh
open http://127.0.0.1:3000/
```

If you don't see any data, please wait a few minutes.
