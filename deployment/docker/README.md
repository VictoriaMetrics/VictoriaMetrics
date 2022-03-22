# Docker compose environment for VictoriaMetrics

To spin-up VictoriaMetrics, vmagent, vmalert, Alertmanager and Grafana run the following command:

`docker-compose up`

For clustered version check [docker compose in cluster branch](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster/deployment/docker).

## VictoriaMetrics

VictoriaMetrics will be accessible on the following ports:

* `--graphiteListenAddr=:2003`
* `--opentsdbListenAddr=:4242`
* `--httpListenAddr=:8428`

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

Default credential:

* login - `admin`
* password - `admin`

Grafana is provisioned by default with following entities:

* VictoriaMetrics datasource
* Prometheus datasource
* VictoriaMetrics overview dashboard
