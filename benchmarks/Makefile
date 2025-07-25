# Time Series Benchmark Suite (TSBS) for VictoriaMetrics. This command runs a
# complete benchmark cycle:
#
# 1. Builds TSBS tools
# 2. Generates sample time series data
# 3. Loads data into VictoriaMetrics
# 4. Generates benchmark queries
# 5. Runs benchmark queries against VictoriaMetrics
#
# The default parameters below are chosen based on the desired scale and time
# range. With these parameters, the benchmark will generate 1 billion samples:
#
# - There are 100K instances. Each instance emits 10 unique metrics. Therefore,
#   and 100K instances emit 1M unique metrics.
# - Within the data file, each line contains 10 samples from one instance
# - Metrics are emitted every 10s interval and there are 3600*24 / 80 = ~1K
#   10s intervals within 24 hours
# - Total number of lines, therefore: 100K machines × ~1K intervals = ~100M
# - And total number of samples: 1M metrics × ~1K intervals = ~1B
#
# The command expects a VictoriaMetrics instance running at
# http://localhost:8428. Use TSBS_WRITE_URL and TSBS_READ_URL to override the
# address.
#
# Adjust TSBS_SCALE to increase/decrease both ingestion and query load.
# Adjust TSBS_WORKERS to control concurrency. It should ideally match the
# number of CPU cores on your VM instance for optimal performance
#
# For accurate benchmark results, run this command on a separate machine from
# VictoriaMetrics since gunzipping and query processing are CPU-intensive
# operations that can impact results when run on the same machine
#
# See https://github.com/timescale/tsbs/blob/master/docs/victoriametrics.md
# for details
tsbs: tsbs-build tsbs-generate-data tsbs-load-data tsbs-generate-queries tsbs-run-queries

TSBS_SCALE := 100000
# If GNU date is available, use it; otherwise, fall back to the standard date command
# User can install GNU date on macOS via `brew install coreutils`
DATE_CMD := $(shell which gdate 2>/dev/null || echo date)
TSBS_START := $(shell $(DATE_CMD) -u -d "1 day ago 00:00:00" +"%Y-%m-%dT%H:%M:%SZ")
TSBS_END   := $(shell $(DATE_CMD) -u -d "00:00:00" +"%Y-%m-%dT%H:%M:%SZ")
TSBS_STEP := 80s
TSBS_QUERIES := 1000
TSBS_WORKERS := 4
TSBS_DATA_FILE := /tmp/tsbs-data-$(TSBS_SCALE)-$(TSBS_START)-$(TSBS_END)-$(TSBS_STEP).gz
TSBS_QUERY_FILE := /tmp/tsbs-queries-$(TSBS_SCALE)-$(TSBS_START)-$(TSBS_END)-$(TSBS_QUERIES).gz
TSBS_LOAD_RESULT_CSV_FILE := /tmp/tsbs-load-$(TSBS_SCALE)-$(TSBS_START)-$(TSBS_END)-$(TSBS_STEP).csv
TSBS_LOAD_RESULT_CSV_FILE_COMPARE ?=
TSBS_PLOT_SCRIPT := $(shell pwd)/benchmarks/plot-load.sh

# For cluster setup use http://vminsert:8480/insert/0/influx/write
TSBS_WRITE_URLS := http://localhost:8428/write
# For cluster setup use http://vmselect:8481/select/0/prometheus
TSBS_READ_URLS := http://localhost:8428
TSBS_METRICS_URL := http://localhost:8428/metrics

# Build TSBS tools
tsbs-build:
	test -d /tmp/tsbs || (git clone https://github.com/timescale/tsbs.git /tmp/tsbs && \
		cd /tmp/tsbs/cmd/tsbs_generate_data && GOBIN=/tmp/tsbs/bin go install && \
		cd /tmp/tsbs/cmd/tsbs_generate_queries && GOBIN=/tmp/tsbs/bin go install && \
		cd /tmp/tsbs/cmd/tsbs_load_victoriametrics && GOBIN=/tmp/tsbs/bin go install && \
		cd /tmp/tsbs/cmd/tsbs_run_queries_victoriametrics && GOBIN=/tmp/tsbs/bin go install)

# Generate sample time series data
tsbs-generate-data:
	test -f $(TSBS_DATA_FILE) || /tmp/tsbs/bin/tsbs_generate_data \
		--format=victoriametrics \
		--use-case=cpu-only  \
		--seed=8428 \
		--scale=$(TSBS_SCALE) \
		--timestamp-start=$(TSBS_START) \
		--timestamp-end=$(TSBS_END) \
		--log-interval=$(TSBS_STEP) \
		| gzip > $(TSBS_DATA_FILE)

# Load data into VictoriaMetrics
tsbs-load-data:
	cat $(TSBS_DATA_FILE) | gunzip | /tmp/tsbs/bin/tsbs_load_victoriametrics --workers=$(TSBS_WORKERS) --urls=$(TSBS_WRITE_URLS) > $(TSBS_LOAD_RESULT_CSV_FILE) 2>/dev/tty
	curl -s $(TSBS_METRICS_URL) | grep \
		-e process_cpu_seconds_user_total \
		-e process_cpu_seconds_system_total \
		-e process_cpu_seconds_total \
		-e process_resident_memory_peak_bytes \
		-e process_resident_memory_bytes \
		-e process_io_read_bytes_total \
		-e process_io_written_bytes_total || true


# Generate benchmark queries
tsbs-generate-queries:
	test -f $(TSBS_QUERY_FILE) || /tmp/tsbs/bin/tsbs_generate_queries \
		--format=victoriametrics \
		--use-case=cpu-only \
		--seed=8428 \
		--scale=$(TSBS_SCALE) \
		--timestamp-start=$(TSBS_START) \
		--timestamp-end=$(TSBS_END) \
		--query-type=cpu-max-all-8 \
		--queries=1000 \
		| gzip > $(TSBS_QUERY_FILE)

# Run benchmark queries against VictoriaMetrics
tsbs-run-queries:
	cat $(TSBS_QUERY_FILE) | gunzip | /tmp/tsbs/bin/tsbs_run_queries_victoriametrics --workers=$(TSBS_WORKERS) --urls=$(TSBS_READ_URLS)

# Build graphs from the load results
tsbs-plot-load:
	test -f $(TSBS_LOAD_RESULT_CSV_FILE) || (echo "Run tsbs-load-data first" && exit 1)
	test -x $(TSBS_PLOT_SCRIPT) || chmod +x $(TSBS_PLOT_SCRIPT) && \
    $(TSBS_PLOT_SCRIPT) $(TSBS_LOAD_RESULT_CSV_FILE) $(TSBS_LOAD_RESULT_CSV_FILE_COMPARE)
