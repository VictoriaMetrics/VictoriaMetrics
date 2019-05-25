# All these commands must run from repository root.

victoria-metrics:
	GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics ./app/victoria-metrics

victoria-metrics-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker

package-victoria-metrics:
	APP_NAME=victoria-metrics \
	$(MAKE) package-via-docker

publish-victoria-metrics:
	APP_NAME=victoria-metrics $(MAKE) publish-via-docker

run-victoria-metrics:
	mkdir -p victoria-metrics-data
	DOCKER_OPTS='-v $(shell pwd)/victoria-metrics-data:/victoria-metrics-data' \
	APP_NAME=victoria-metrics \
	ARGS='-graphiteListenAddr=:2003 -opentsdbListenAddr=:4242 -retentionPeriod=12 -search.maxUniqueTimeseries=1000000 -search.maxQueryDuration=10m' \
	$(MAKE) run-via-docker

victoria-metrics-arm:
	CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-arm ./app/victoria-metrics
