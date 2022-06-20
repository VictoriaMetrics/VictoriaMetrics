# Series modification

## Scenario

VictoriaMetrics doesn't support direct data modification, since it uses immutable data structures and such operations may significantly reduce system performance.

The new series update API should provide a workaround for this issue. API allows overriding existing Timeseries data points at runtime during select requests.

Following operations supported:
- add data points.
- remove data points.
- modify data points.


Note this is a low-level feature, data modification could be done with scripts, vmctl, or `VMUI` in future releases.

## Examples


### Setup env

It's expected, that you have configured VictoriaMetrics cluster, vminsert, and vmselect components reachable from your computer.

I'll work with the following data set, which was exported with a call to the export API:
```text
curl localhost:8481/select/0/prometheus/api/v1/export -g -d 'match[]={__name__="vmagent_rows_inserted_total"}' -d 'start=1658164769' -d 'end=1658165291'

{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdbhttp","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"vmimport","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"native","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"prometheus","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"graphite","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"csvimport","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"promremotewrite","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"datadog","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
```

For better usability, it could be exported to a file on disk and modified via a preferred text editor.


### Modify data points

#### change values

Let's say, during ingestion some error happened and producer incorrectly ingest value `0` for timestamp 1658164969982 and `Prometheus` and `influx` types:
```text
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"prometheus","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164969982,1658164979982,1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165209982,1658165219982,1658165229982,1658165239982,1658165249982,1658165259982,1658165261982]}
```

we have to modify these values to correct `0` and send update request to the `vminsert` API:
```text
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"prometheus","instance":"localhost:8429"},"values":[0],"timestamps":[1658164969982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0],"timestamps":[1658164969982]}
```

data points could be also updated at time range if actual timestamp is not known. For instance, [1658164969972,1658164969982,1658164969992] timestamp range overwrite values for given timestamps and drops any timestamps at a given time range.

Save 2 series above into the file `incorrect_value_modification.txt` and execute API request with the curl command:
```text
curl localhost:8480/insert/0/prometheus/api/v1/update/series -T incorrect_values_modification.txt
```

Check series modification output:
```text
curl localhost:8481/select/0/prometheus/api/v1/export -g -d 'match[]={__name__="vmagent_rows_inserted_total",type=~"prometheus|influx"}' -d 'start=1658164969' -d 'end=1658164989'

{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"prometheus","instance":"localhost:8429"},"values":[0,0,0],"timestamps":[1658164969982,1658164979982,1658164987982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0,0,0],"timestamps":[1658164969982,1658164979982,1658164987982]}
```

#### Add missing timestamps

Missing timestamps could be added in the same way, specify needed timestamps with needed values at correct array indexes.

### Delete data points at time range

For example data set we have following time range from `1658164969982` to `1658165261982`.
Data points inside time range can be removed by skipping timestamps and time range, which must be removed.
For example, if timestamps from `1658164999982` until `1658165099982` must be removed, skip all timestamps between it:
```text
# exclude variant
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdbhttp","instance":"localhost:8429"},"values":[0,0,0,0],"timestamps":[1658164989982,1658164999982,1658165099982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429"},"values":[0,0,0,0],"timestamps":[1658164989982,1658164999982,1658165099982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"vmimport","instance":"localhost:8429"},"values":[0,0,0,0],"timestamps":[1658164989982,16581649999821658165099982,1658165109982]}

# include variant
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdbhttp","instance":"localhost:8429"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"vmimport","instance":"localhost:8429"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
```

saved on of variants into the file `delete_datapoints_range.txt` and execute following request to the API:
```text
curl localhost:8480/insert/0/prometheus/api/v1/update/series -T delete_datapoints_range.txt
```

Check output:
```text
 curl localhost:8481/select/0/prometheus/api/v1/export -g -d 'match[]={__name__="vmagent_rows_inserted_total",type=~"influx|opentsdb"}' -d 'start=1658164989' -d 'end=1658165209'
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165207982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164989982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165207982]}
```

As you see, series with `opentsdb` type has less data points than `influx`, since data was deleted at time range.

### Observing changes

Changes could de check by export api request with special query params `reduce_mem_usage=true` and `extra_filters={__generation_id!=""}`.

Let's observe changes from previous steps:
```text
curl localhost:8481/select/0/prometheus/api/v1/export -g -d 'match[]={__name__="vmagent_rows_inserted_total"}' -d 'reduce_mem_usage=true' -d 'extra_filters={__generation_id!=""}'

{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"prometheus","instance":"localhost:8429","__generation_id":"1658166029893830000"},"values":[0,0],"timestamps":[1658164969982,1658164979982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429","__generation_id":"1658166029893830000"},"values":[0,0],"timestamps":[1658164969982,1658164979982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"vmimport","instance":"localhost:8429","__generation_id":"1658167040791371000"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429","__generation_id":"1658167040791371000"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdbhttp","instance":"localhost:8429","__generation_id":"1658167040791371000"},"values":[0,0],"timestamps":[1658164989982,1658165109982]}
```


### Rollback update operations

Changes could be undone with metrics DELETE API, you have to specify correct `__generation_id`.
For example, rollback timestamps delete:
```text
curl http://localhost:8481/delete/0/prometheus/api/v1/admin/tsdb/delete_series -g -d 'match={__generation_id="1658167040791371000"}'
```

Check that changes were rolled back:
 ```text
curl localhost:8481/select/0/prometheus/api/v1/export -g -d 'match[]={__name__="vmagent_rows_inserted_total",type=~"influx|opentsdb"}' -d 'start=1658164989' -d 'end=1658165209'
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"influx","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165207982]}
{"metric":{"__name__":"vmagent_rows_inserted_total","job":"vminsert","type":"opentsdb","instance":"localhost:8429"},"values":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"timestamps":[1658164989982,1658164999982,1658165009982,1658165019982,1658165029982,1658165039982,1658165049982,1658165059982,1658165069982,1658165079982,1658165089982,1658165099982,1658165109982,1658165119982,1658165129982,1658165139982,1658165149982,1658165159982,1658165169982,1658165179982,1658165189982,1658165199982,1658165207982]}
```