<img  text-align="center" alt="Victoria Metrics" src="logo.png">

## VictoriaMetrics - the best long-term remote storage for Prometheus


### VictoriaMetrics features

- Full [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) support. Additionally, VictoriaMetrics extends PromQL with useful features. See [Extended PromQL](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/ExtendedPromQL) for more details.
- Simple configuration. Just copy-n-paste remote storage URL to Prometheus config and that's it! See [Quick Start](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/Quick-Start) for more info.
- Reduced operational overhead. Prometheus local storage retention may be set to the minimum possible value when using VictoriaMetrics remote storage. This effectively makes Prometheus stateless, so it may be run as a stateless service in Kubernetes.
- Insertion rate scales to millions of metric values per second.
- Storage scales to millions of metrics with trillions of metric values.
- Wide range of retention periods - from 1 month to 5 years. Users may create different projects (aka `storage namespaces`) with different retention periods.
- Fast query engine. It excels on heavy queries over thousands of metrics with millions of metric values.
- The same remote storage URL may be used by multiple Prometheus instances collecting distinct metric sets, so all these metrics may be used in a single query (aka `global querying view`). This works ideally for multiple Prometheus instances located in different subnetworks / datacenters.


### Useful links

* [Site](https://victoriametrics.com/)
* [`WITH` templates playground](https://play.victoriametrics.com/promql/expand-with-exprs)
* [Grafana playground](http://play-grafana.victoriametrics.com:3000/d/4ome8yJmz/node-exporter-on-victoriametrics-demo)
* [Docs](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki)
* [FAQ](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/FAQ)
* [Issues](https://github.com/VictoriaMetrics/VictoriaMetrics/issues)
* [Google group](https://groups.google.com/forum/#!forum/victoriametrics)
* [Creating the best remote storage for Prometheus](https://medium.com/devopslinks/victoriametrics-creating-the-best-remote-storage-for-prometheus-5d92d66787ac) - an article with technical details about VictoriaMetrics.
* [Docker images](https://hub.docker.com/r/valyala/victoria-metrics/) and the corresponding [binaries](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) for single-server VictoriaMetrics


### Victoria Metrics Logo

[Zip](VM_logo.zip) contains three folders with different image orientation (main color and inverted version).

Files included in each folder:

* 2 JPEG Preview files
* 2 PNG Preview files with transparent background
* 2 EPS Adobe Illustrator EPS10 files


#### Logo Usage Guidelines

##### Font used: 

* Lato Black 
* Lato Regular

##### Color Palette:

* HEX [#110f0f](https://www.color-hex.com/color/110f0f) 
* HEX [#ffffff](https://www.color-hex.com/color/ffffff)

#### We kindly ask:

- Please don't use any other font instead of suggested.
- There should be sufficient clear space around the logo.
- Do not change spacing, alignment, or relative locations of the design elements.
- Do not change the proportions of any of the design elements or the design itself. You    may resize as needed but must retain all proportions.



