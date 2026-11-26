# Docker Compose file for "vmanomaly integration" guide

Please read the "vmanomaly integration" guide first - [Anomaly Detection and Alerting Setup](https://docs.victoriametrics.com/anomaly-detection/guides/guide-vmanomaly-vmalert/)

The v1.33.1 example passes `-eula` to accept the [Enterprise Software Agreement](https://victoriametrics.com/legal/esa/). See [licensing](https://docs.victoriametrics.com/anomaly-detection/quickstart/#licensing) for compatibility with earlier versions.

## How to run 

1. Review the configuration and ESA before starting the example.
1. Run

```sh 
docker compose up -d  
```
1. Open Grafana on  `http://127.0.0.1:3000/`
```sh
open http://127.0.0.1:3000/
```

If you don't see any data, please wait a few minutes.
