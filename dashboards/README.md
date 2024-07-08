# VictoriaMetrics dashboards

The directory contains the official list of Grafana dashboards for VictoriaMetrics components.
The `vm` folder contains copies of the listed dashboards but alternated to use 
[VictoriaMetrics datasource](https://github.com/VictoriaMetrics/victoriametrics-datasource).

The listed dashboards can be found on [Grafana website](https://grafana.com/orgs/victoriametrics/dashboards).

When making changes to the dashboards in `dashboards` folder, don't forget to call `make dashboards-sync`
and sync changes to [Grafana website](https://grafana.com/orgs/victoriametrics/dashboards).