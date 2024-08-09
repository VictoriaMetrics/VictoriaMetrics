[vmagent](https://docs.victoriametrics.com/vmagent) and [single-node VictoriaMetrics](https://docs.victoriametrics.com)
can aggregate incoming [samples](https://docs.victoriametrics.com/keyconcepts#raw-samples) in streaming mode by time and by labels before data is written to remote storage
(or local storage for single-node VictoriaMetrics).
The aggregation is applied to all the metrics received via any [supported data ingestion protocol](https://docs.victoriametrics.com#how-to-import-time-series-data)
and/or scraped from [Prometheus-compatible targets](https://docs.victoriametrics.com#how-to-scrape-prometheus-exporters-such-as-node-exporter)
after applying all the configured [relabeling stages](https://docs.victoriametrics.com/vmagent#relabeling).

_By default, stream aggregation ignores timestamps associated with the input [samples](https://docs.victoriametrics.com/keyconcepts#raw-samples).
It expects that the ingested samples have timestamps close to the current time. See [how to ignore old samples](./configuration/#ignoring-old-samples)._

## Next steps
{{% section %}}
