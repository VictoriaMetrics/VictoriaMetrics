---
sort: 12
---

# Quick Start

The following commands download the latest available [Docker image of VictoriaMetrics](https://hub.docker.com/r/victoriametrics/victoria-metrics) and start it at port 8428, while storing the ingested data at `victoria-metrics-data` subdirectory under the current directory:

```bash
docker pull victoriametrics/victoria-metrics:latest
docker run -it --rm -v `pwd`/victoria-metrics-data:/victoria-metrics-data -p 8428:8428 victoriametrics/victoria-metrics:latest
```

Open `http://localhost:8428` in web browser and read [these docs](https://docs.victoriametrics.com/#operation).

VictoriaMetrics is also available in binaries (see [this page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)) and in source code (see [how to build VictoriaMetrics from sources](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-build-from-sources)).

There are also the following versions of VictoriaMetrics available:
* [VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html) - horizontally scalable VictoriaMetrics, which scales to multiple nodes.
* [Managed VictoriaMetrics at AWS](https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc).
