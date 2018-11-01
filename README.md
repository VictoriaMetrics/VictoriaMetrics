## VictoriaMetrics - the best remote storage for Prometheus


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
