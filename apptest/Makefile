TS_START := $(shell date -u -v-1d +"%Y-%m-%dT%H:%M:%SZ")
TS_END   := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

tsbs-build:
	test -d /tmp/tsbs || git clone https://github.com/timescale/tsbs.git /tmp/tsbs && \
	cd /tmp/tsbs/cmd/tsbs_generate_data && GOBIN=/tmp/tsbs/bin go install && \
	cd ../tsbs_generate_queries && GOBIN=/tmp/tsbs/bin go install && \
    cd ../tsbs_load_victoriametrics && GOBIN=/tmp/tsbs/bin go install && \
    cd ../tsbs_run_queries_victoriametrics && GOBIN=/tmp/tsbs/bin go install

tsbs-generate-data:
	test -f /tmp/tsbs-data.gz || /tmp/tsbs/bin/tsbs_generate_data \
	--format=victoriametrics \
	--use-case=devops  \
	--seed=8428 \
	--scale=100 \
	--timestamp-start=$(TS_START) \
	--timestamp-end=$(TS_END) \
	--log-interval=10s \
	| gzip > /tmp/tsbs-data.gz;

tsbs-load-data:
	cat /tmp/tsbs-data.gz | gunzip | /tmp/tsbs/bin/tsbs_load_victoriametrics --urls=http://localhost:8428/influx/write

tsbs-generate-queries:
	test -f /tmp/queries.gz || /tmp/tsbs/bin/tsbs_generate_queries \
	--format=victoriametrics \
	--queries=1000 \
	--query-type=cpu-max-all-8 \
	--scale=4000 \
	--seed=8428 \
	--timestamp-start=$(TS_START) \
	--timestamp-end=$(TS_END) \
	--use-case=cpu-only \
	| gzip > /tmp/queries.gz; \

tsbs-run-queries:
	cat /tmp/queries.gz | gunzip | /tmp/tsbs/bin/tsbs_run_queries_victoriametrics
