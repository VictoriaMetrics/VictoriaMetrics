---
weight: 1
title: Try it
menu:
  docs:
    identifier: vm-try-it
    parent: victoriametrics
    weight: 1
tags:
  - metrics
  - guide
aliases:
- /Try-It.html
- /try-it/index.html
- /try-it/
---
This page shows the fastest way to try VictoriaMetrics on your own machine: download a single binary,
start it with one command, and see live metrics in the built-in UI in a couple of minutes.
No Docker, no configuration files and no extra components are required.

If you'd rather not install anything at all, try [Playgrounds](https://docs.victoriametrics.com/playgrounds/) -
a list of publicly available playgrounds for VictoriaMetrics software.

To run VictoriaMetrics on your own machine, the only thing needed is its binary.

## Step 1: Download the binary

Create a directory for this test drive, so all the files created along the way stay in one place:

```sh
mkdir vm-try-it && cd vm-try-it
```

Download the `victoria-metrics-<os>-<arch>-<version>.tar.gz` archive for your OS and architecture
from the [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
and unpack it. It contains a single `victoria-metrics-prod` binary.

For example, on Linux with `amd64` architecture:

```sh
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.146.0/victoria-metrics-linux-amd64-v1.146.0.tar.gz
tar xzf victoria-metrics-linux-amd64-v1.146.0.tar.gz
```

The binary is self-contained and requires no installation - it is ready to run as is.

## Step 2: Start VictoriaMetrics

Starting VictoriaMetrics is as simple as executing the binary, with no arguments at all.
But since it is nicer to have some data to explore right after the start, let's also enable self-scraping
via the `-selfScrapeInterval` command-line flag, so VictoriaMetrics collects metrics about itself:

```sh
./victoria-metrics-prod -selfScrapeInterval=10s
```

VictoriaMetrics prints a couple of dozen log lines on start, describing the storage, caches
and memory limits it sets up. Look for these two lines confirming that it is up and running:

```sh
2026-07-10T16:55:06.615Z    info    app/victoria-metrics/main.go:102    started VictoriaMetrics in 0.019 seconds
...
2026-07-10T16:55:06.615Z    info    lib/httpserver/httpserver.go:148    started server at http://0.0.0.0:8428/
```

That's it - VictoriaMetrics is running, listening on port `8428` and scraping its own metrics
(CPU and memory usage, the number of stored metrics, request rates, etc.) every 10 seconds.
If you list the `vm-try-it` directory, you can see a new `victoria-metrics-data` directory
created next to the binary - this is where the collected data is stored.

Now that metrics are being collected, it's time to look at them.

## Step 3: Explore the metrics

Open [http://localhost:8428/vmui](http://localhost:8428/vmui) in your browser to access [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) -
the built-in UI for querying and graphing metrics. Self-scraped metrics become queryable within ~30 seconds after the start.

Try the following:

* Open the [metrics explorer](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#metrics-explorer)
  at [http://localhost:8428/vmui/#/metrics](http://localhost:8428/vmui/#/metrics) to browse all collected metrics;
* Enter a query in the input field at [http://localhost:8428/vmui](http://localhost:8428/vmui) and press `Enter`. For example:
  * `process_resident_memory_bytes` - memory used by VictoriaMetrics;
  * `rate(process_cpu_seconds_total)` - its CPU usage;
  * `vm_rows{type=~"storage/.+"}` - the number of stored [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples).

Queries are written in [MetricsQL](https://docs.victoriametrics.com/victoriametrics/metricsql/) -
a PromQL-compatible query language, so any PromQL query works here as well.

So far the only available metrics are the ones VictoriaMetrics reports about itself - let's collect something more interesting.

## Step 4 (optional): Collect metrics from your system

Self-scraped metrics only describe VictoriaMetrics itself. To monitor your machine (CPU, memory, disk, network),
run [node_exporter](https://github.com/prometheus/node_exporter) - the standard Prometheus exporter for host metrics -
and let VictoriaMetrics scrape it. Single-node VictoriaMetrics has a built-in
[Prometheus-compatible scraper](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter),
so no other components are needed.

1. Download the `node_exporter-<version>.<os>-<arch>.tar.gz` archive for your OS and architecture
   from the [releases page](https://github.com/prometheus/node_exporter/releases/latest) and unpack it.
   For example, on Linux with `amd64` architecture:

   ```sh
   wget https://github.com/prometheus/node_exporter/releases/download/v1.12.0/node_exporter-1.12.0.linux-amd64.tar.gz
   tar xzf node_exporter-1.12.0.linux-amd64.tar.gz
   ```

   Unlike VictoriaMetrics, it unpacks into its own directory. Start it from there:

   ```sh
   ./node_exporter-1.12.0.linux-amd64/node_exporter
   ```

   It exposes host metrics at [http://localhost:9100/metrics](http://localhost:9100/metrics).

1. Create a `scrape.yaml` file with the following contents:

   ```yaml
   scrape_configs:
   - job_name: node-exporter
     static_configs:
     - targets:
       - localhost:9100
   ```

1. Restart VictoriaMetrics with the `-promscrape.config` command-line flag pointing to this file:

   ```sh
   ./victoria-metrics-prod -selfScrapeInterval=10s -promscrape.config=scrape.yaml
   ```

Check [http://localhost:8428/targets](http://localhost:8428/targets) - the `node-exporter` target should have `state: up`.
The target shows up as `state: down` until the first scrape happens, which can take some seconds -
just reload the page a bit later.
Now query host metrics in [vmui](http://localhost:8428/vmui). For example:

* `node_memory_MemAvailable_bytes` - available memory on your machine;
* `100 - avg(rate(node_cpu_seconds_total{mode="idle"})) * 100` - overall CPU usage in percent.

See [scrape config examples](https://docs.victoriametrics.com/victoriametrics/scrape_config_examples/) for more advanced scrape configurations.

Scraping pulls metrics from targets, but it is not the only way to get data in - metrics can also be pushed directly.

## Step 5 (optional): Push your own metrics

VictoriaMetrics also accepts metrics pushed via [many popular protocols](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model).
For example, insert a measurement using the InfluxDB line protocol with a plain `curl` command:

```sh
curl -d 'room_temperature,room=kitchen value=21.5' http://localhost:8428/write
```

Then query `room_temperature_value` in [vmui](http://localhost:8428/vmui) to see it.
Take into account that freshly ingested data can take up to 30 seconds to show up in query results,
so don't worry if it doesn't appear immediately - just retry the query a bit later.

Once you're done experimenting, tidying everything up takes a single command.

## Cleanup

Stop VictoriaMetrics and node_exporter with `Ctrl+C` in their respective terminals. All the collected data lives in the `victoria-metrics-data` directory -
delete it if you want to start from scratch. To remove all traces of this test drive,
delete the whole `vm-try-it` directory created at [step 1](#step-1-download-the-binary).

This test drive only scratches the surface of what VictoriaMetrics can do.

## Next steps

* Follow the [Quick start](https://docs.victoriametrics.com/victoriametrics/quick-start/) guide for other installation
  options (Docker, Helm, Kubernetes operator, VictoriaMetrics Cloud) and production recommendations;
* Learn the [key concepts](https://docs.victoriametrics.com/victoriametrics/keyconcepts/) of writing and querying data.

Or keep trying VictoriaMetrics with your own setup:

* Connect [Grafana](https://docs.victoriametrics.com/victoriametrics/integrations/grafana/) to build dashboards
  on top of the collected metrics;
* Instrument your application with [OpenTelemetry](https://docs.victoriametrics.com/victoriametrics/integrations/opentelemetry/)
  and send its metrics directly to VictoriaMetrics;
* Point your existing [Prometheus](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/)
  to VictoriaMetrics as a remote write target and long-term storage;
* Explore the full list of [integrations](https://docs.victoriametrics.com/victoriametrics/integrations/)
  for other systems VictoriaMetrics works with.
