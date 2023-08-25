# VictoriaMetrics dashboards

The directory contains the official list of Grafana dashboards for VictoriaMetrics components.
The `vm` folder contains copies of the listed dashboards but alternated to use 
[VictoriaMetrics datasource](https://github.com/VictoriaMetrics/grafana-datasource).

The listed dashboards can be found on [Grafana website](https://grafana.com/orgs/victoriametrics/dashboards).

When making changes to the dashboards upstream, don't forget to call `make dashboards-sync`.