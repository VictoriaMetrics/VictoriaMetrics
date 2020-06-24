# Sample size calculations

These calculations are for the “Lowest sample size” graph at https://victoriametrics.com/ .

How many metrics can be stored in 2tb disk for 2 years?

Seconds in 2 years:
2 years * 365 days * 24 hours * 60 minutes * 60 seconds = 63072000 seconds

Resolution = 1 point per 10 second

That means each metric will contain 6307200 points.

2tb disk contains
2 (tb) * 1024 (gb) * 1024 (mb) * 1024 (kb) * 1024 (b)  = 2199023255552 bytes

# VictoriaMetrics
Based on production data from our customers, sample size is 0.4 byte
That means one metric with 10 seconds resolution will need
6307200 points * 0.4 bytes/point = 2522880 bytes or 2.4 megabytes.
Calculation for number of metrics can be stored in 2 tb disk:
2199023255552 (disk size) / 2522880 (one metric for 2 year) = 871632 metrics
So in 2tb we can store 871 632 metrics

# Graphite
Based on https://m30m.github.io/whisper-calculator/ sample size of graphite metrics is 12b + 28b for each metric
That means, one metric with 10 second resolution will need 75686428 bytes or 72.18 megabytes
Calculation for number of metrics can be stored in 2 tb disk:
2199023255552 / 75686428 = 29 054 metrics

# OpenTSDB
Let's check official openTSDB site
http://opentsdb.net/faq.html
16 bytes of HBase overhead, 3 bytes for the metric, 4 bytes for the timestamp, 6 bytes per tag, 2 bytes of OpenTSDB overhead, up to 8 bytes for the value. Integers are stored with variable length encoding and can consume 1, 2, 4 or 8 bytes.
That means, one metric with 10 second resolution will need
6307200 * (1 + 4) + 3 + 16 + 2 = 31536021 bytes or 30 megabytes in the best scenario and
6307200 * (8 + 4) + 3 + 16 + 2 = 75686421 bytes or 72 megabytes in the worst scenario.

Calculation for number of metrics can be stored in 2 tb disk:

2199023255552 / 31536021  = 69 730 metrics for best scenario
2199023255552 / 75686421 = 29 054 metrics for worst scenario

Also, openTSDB allows to use compression
" LZO is able to achieve a compression factor of 4.2x "
So, let's multiply numbers on 4.2
69 730 * 4,2 = 292 866 metrics for best scenario
29 054 * 4,2 = 122 026 metrics for worst scenario
# m3db
Let's look at official m3db site https://m3db.github.io/m3/m3db/architecture/engine/
They can achieve a sample size of 1.45 bytes/datapoint
That means, one metric with 10 second resolution will need 9145440 bytes or 8,72177124 megabytes
Calculation for number of metrics can be stored in 2 tb disk:
2199023255552 / 9145440  = 240 450 metrics

# InfluxDB
Based on official influxDB site https://docs.influxdata.com/influxdb/v1.8/guides/hardware_sizing/#bytes-and-compression
"Non-string values require approximately three bytes". That means, one metric with 10 second resolution will need
6307200 * 3 = 18921600 bytes or 18 megabytes
Calculation for number of metrics can be stored in 2 tb disk:

2199023255552 / 18921600 = 116 217 metrics

# Prometheus
Let's check official site: https://prometheus.io/docs/prometheus/latest/storage/
"On average, Prometheus uses only around 1-2 bytes per sample."
That means, one metric with 10 second resolution will need
6307200 * 1 = 6307200 bytes in best scenario
6307200 * 2 = 12614400 bytes in worst scenario.

Calculation for number of metrics can be stored in 2 tb disk:

2199023255552 / 6307200  = 348 652 metrics for the best case
2199023255552 / 12614400 = 174 326 metrics for the worst cases
