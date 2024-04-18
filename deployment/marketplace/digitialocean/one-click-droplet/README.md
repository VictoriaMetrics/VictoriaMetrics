## Application summary

VictoriaMetrics is a fast and scalable open source time series database and monitoring solution.

## Description

VictoriaMetrics is a free [open source time series database](https://en.wikipedia.org/wiki/Time_series_database) (TSDB) and monitoring solution, designed to collect, store and process real-time metrics.

It supports the [Prometheus](https://en.wikipedia.org/wiki/Prometheus_(software)) pull model and various push protocols ([Graphite](https://en.wikipedia.org/wiki/Graphite_(software)), [InfluxDB](https://en.wikipedia.org/wiki/InfluxDB), OpenTSDB) for data ingestion. It is optimized for storage with high-latency IO, low IOPS and time series with [high churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate).

For reading the data and evaluating alerting rules, VictoriaMetrics supports the PromQL, [MetricsQL](https://docs.victoriametrics.com/metricsql/) and Graphite query languages. VictoriaMetrics Single is fully autonomous and can be used as a long-term storage for time series.

[VictoriaMetrics Single](https://docs.victoriametrics.com/single-server-victoriametrics/) = Hassle-free monitoring solution. Easily handles 10M+ of active time series on a single instance. Perfect for small and medium environments.

## Getting started after deploying VictoriaMetrics Single

### Config

VictoriaMetrics configuration is located at `/etc/victoriametrics/single/scrape.yml` on the droplet.
This One Click app uses 8428, 2003, 4242 and 8089 ports to accept metrics from different protocols. It's recommended to disable ports for protocols which are not needed. [Ubuntu firewall](https://help.ubuntu.com/community/UFW) can be used to easily disable access for specific ports.

### Scraping metrics

VictoriaMetrics supports metrics scraping in the same way as Prometheus does. Check the configuration file to edit scraping targets. See more details about scraping at [How to scrape Prometheus exporters](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter).

### Sending metrics

Besides scraping, VictoriaMetrics accepts write requests for various ingestion protocols. This One Click app supports the following protocols:

- [Datadog](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-datadog-agent), [Influx (telegraph)](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf), [JSON](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-data-in-json-line-format), [CSV](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-csv-data), [Prometheus](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format)  on port :8428
- [Graphite (statsd)](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-graphite-compatible-agents-such-as-statsd) on port :2003 tcp/udp
- [OpenTSDB](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-send-data-from-opentsdb-compatible-agents) on port :4242
- Influx (telegraph) on port :8089 tcp/udp

See more details and examples in [official documentation](https://docs.victoriametrics.com/single-server-victoriametrics/).

### UI

VictoriaMetrics provides a [User Interface (UI)](https://docs.victoriametrics.com/single-server-victoriametrics/#vmui) for query troubleshooting and exploration. The UI is available at `http://your_droplet_public_ipv4:8428/vmui`. It lets users explore query results via graphs and tables.

To check it, open the following in your browser `http://your_droplet_public_ipv4:8428/vmui` and then enter `vm_app_uptime_seconds` to the Query Field to Execute the Query.

Run the following command to query and retrieve a result from VictoriaMetrics Single with `curl`:

```console
curl -sg http://your_droplet_public_ipv4:8428/api/v1/query_range?query=vm_app_uptime_seconds | jq
```

### Accessing

Once the Droplet is created, you can use DigitalOcean's web console to start a session or  SSH directly to the server as root:

```console
ssh root@your_droplet_public_ipv4
```
