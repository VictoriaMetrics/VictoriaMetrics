# Docker compose environment for VictoriaMetrics

Docker compose environment for VictoriaMetrics includes VictoriaMetrics components,
[Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/) 
and [Grafana](https://grafana.com/).

For starting the docker-compose environment ensure you have docker installed and running and access to the Internet.
All commands should be executed from the root directory of this repo.

To spin-up environment for single server VictoriaMetrics run the following command :
```
make docker-single-up
```

To shutdown the docker compose environment for single server run the following command:
```
make docker-single-down
```

For cluster version the command will be the following:
```
make docker-cluster-up
```

To shutdown the docker compose environment for cluster version run the following command:
```
make docker-cluster-down
```

## VictoriaMetrics single server

VictoriaMetrics will be accessible on the following ports:

* `--graphiteListenAddr=:2003`
* `--opentsdbListenAddr=:4242`
* `--httpListenAddr=:8428`

The communication scheme between components is the following:
* [vmagent](#vmagent) sends scraped metrics to VictoriaMetrics;
* [grafana](#grafana) is configured with datasource pointing to VictoriaMetrics;
* [vmalert](#vmalert) is configured to query VictoriaMetrics and send alerts state
  and recording rules back to it;
* [alertmanager](#alertmanager) is configured to receive notifications from vmalert.

To access `vmalert` via `vmselect`
use link [http://localhost:8428/vmalert](http://localhost:8428/vmalert/).

To access [vmui](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#vmui)
use link [http://localhost:8428/vmui](http://localhost:8428/vmui).

## VictoriaMetrics cluster

VictoriaMetrics cluster environemnt consists of vminsert, vmstorage and vmselect components. vmselect
has exposed port `:8481`, vminsert has exposed port `:8480` and the rest of components are available
only inside of environment.

The communication scheme between components is the following:
* [vmagent](#vmagent) sends scraped metrics to vminsert;
* vminsert forwards data to vmstorage;
* vmselect is connected to vmstorage for querying data;
* [grafana](#grafana) is configured with datasource pointing to vmselect;
* [vmalert](#vmalert) is configured to query vmselect and send alerts state
  and recording rules to vminsert;
* [alertmanager](#alertmanager) is configured to receive notifications from vmalert.

To access `vmalert` via `vmselect` 
use link [http://localhost:8481/select/0/prometheus/vmalert](http://localhost:8481/select/0/prometheus/vmalert/).

To access [vmui](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#vmui) 
use link [http://localhost:8481/select/0/prometheus/vmui](http://localhost:8481/select/0/prometheus/vmui).

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

* `VictoriaMetrics` datasource
* `VictoriaMetrics - cluster` datasource
* `VictoriaMetrics overview` dashboard
* `VictoriaMetrics - cluster` dashboard
* `VictoriaMetrics - vmagent` dashboard
* `VictoriaMetrics - vmalert` dashboard

Remember to pick `VictoriaMetrics - cluster` datasource when viewing `VictoriaMetrics - cluster` dashboard.