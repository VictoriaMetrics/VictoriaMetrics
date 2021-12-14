# vmui

Web UI for VictoriaMetrics

## Docker image build

Run the following command from the root of VictoriaMetrics repository in order to build `victoriametrics/vmui` Docker image:

```
make vmui-release
```

Then run the built image with:

```
docker run --rm --name vmui -p 8080:8080 victoriametrics/vmui
```

Then navigate to `http://localhost:8080` in order to see the web UI.


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

Then navigate to `http://localhost:8428/vmui/`. See [these docs](https://docs.victoriametrics.com/#vmui) for more details.
