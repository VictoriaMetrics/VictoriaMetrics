# vmui

Web UI for VictoriaMetrics

Features:

- configurable Server URL
- configurable time range - every variant have own resolution to show around 30 data points
- query editor has basic highlighting and can be multi-line
- chart is responsive by width
- color assignment for series is automatic
- legend with reduced naming
- tooltips for closest data point
- auto-refresh mode with several time interval presets
- table and raw JSON Query viewer


## Docker image build

Run the following command from the root of VictoriaMetrics repository in order to build `victoriametrics/vmui` Docker image:

```
make vmui-release
```

Then run the built image with:

```
docker run --rm --name vmui -p 8080:8080 victoriametrics/vmui
```

Then naviate to `http://localhost:8080` in order to see the web UI.


## Static build

Run the following command from the root of VictoriaMetrics repository for building `vmui` static contents:

```
make vmui-build
```

The built static contents is put into `app/vmui/packages/vmui/` directory.


## Updating vmui embedded into VictoriaMetrics

Run the following command from the root of VictoriaMetrics repository for updating `vmui` embedded into VictoriaMetrics:

```
make vmui-update
```

This command should update `vmui` static files at `app/vmselect/vmui` directory. Commit changes to these files if needed.

Then build VictoriaMetrics with the following command:

```
make victoria-metrics
```

Then run the built binary with the following command:

```
bin/victoria-metrics -selfScrapeInterval=5s
```

Then navigate to `http://localhost:8428/vmui/`
