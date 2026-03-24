---
weight: 3 
title: VictoriaTraces Playground
menu:
  docs:
    parent: "playgrounds"
    weight: 3
tags:
- victoriatraces
- playground
- monitoring
---

- Try it: <https://play-vtraces.victoriametrics.com/>
- Query language reference: [LogsQL](https://docs.victoriametrics.com/victorialogs/logsql/)

> [!NOTE] Note
> This playground is currently under development, as the main project it is correlated with, VictoriaTraces, is also under development.

VictoriaTraces provides a UI for browsing raw data and Jaeger APIs/Grafana data source for trace visualization. This playground showcases VictoriaTraces, the VictoriaMetrics backend for distributed tracing, and enables trace searching, visualization, and service graph/dependency analysis.

![Screenshot of Grafana](vt-grafana.webp)
<figcaption style="text-align: center; font-style: italic;">Grafana playground showing VictoriaTraces/Jaeger datasource</figcaption>

## What can you do here?

The WebUI provides the following modes for displaying query results:
- Group: results are displayed as a table with rows grouped by stream fields.
- Table: displays query results as a table.
- JSON: displays raw JSON response from the HTTP API.
- Live: displays live tailing results for the given query.

## Distribution

- GitHub: <https://github.com/VictoriaMetrics/VictoriaTraces>

