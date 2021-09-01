# Docker compose environment for VictoriaMetrics

To spin-up VictoriaMetrics cluster, vmagent, vmalert, Alertmanager and Grafana run the following command:

`docker-compose up`

For single version check [docker compose in master branch](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker).

## VictoriaMetrics

VictoriaMetrics cluster in this environment consists of 
vminsert, vmstorage and vmselect components. Only vmselect
has exposed port `:8481` and the rest of components are available
only inside of environment. 
The communication scheme between components is the following:
* [vmagent](#vmagent) sends scraped metrics to vminsert;
* vminsert forwards data to vmstorage;
* vmselect is connected to vmstorage for querying data;
* [grafana](#grafana) is configured with datasource pointing to vmselect;
* [vmalert](#vmalert) is configured to query vmselect and send alerts state
and recording rules to vminsert; 
* [alertmanager](#alertmanager) is configured to receive notifications from vmalert.

## vmagent

vmagent is used for scraping and pushing timeseries to
VictoriaMetrics instance. It accepts Prometheus-compatible
configuration `prometheus.yml` with listed targets for scraping.

[Web interface link](http://localhost:8429/).

## vmalert

vmalert evaluates alerting rules (`alerts.yml`) to track VictoriaMetrics
health state. It is connected with AlertManager for firing alerts,
and with VictoriaMetrics for executing queries and storing alert's state.

[Web interface link](http://localhost:8880/).

## alertmanager

AlertManager accepts notifications from `vmalert` and fires alerts.
All notifications are blackholed according to `alertmanager.yml` config.

[Web interface link](http://localhost:9093/).

## Grafana

To access service open following [link](http://localhost:3000).

Default creds:
* login - `admin`
* password - `admin`

Grafana is provisioned by default with following entities:
* VictoriaMetrics datasource
* Prometheus datasource
* VictoriaMetrics overview dashboard
