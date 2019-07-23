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
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-arm ./app/victoria-metrics

victoria-metrics-arm-prod:
	APP_NAME=victoria-metrics APP_SUFFIX='-arm' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm' $(MAKE) app-via-docker

victoria-metrics-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-arm64 ./app/victoria-metrics

victoria-metrics-arm64-prod:
	APP_NAME=victoria-metrics APP_SUFFIX='-arm64' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm64' $(MAKE) app-via-docker

victoria-metrics-pure:
	GO111MODULE=on CGO_ENABLED=0 go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-pure ./app/victoria-metrics

victoria-metrics-pure-prod:
	APP_NAME=victoria-metrics APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

### Packaging for Debian - amd64
victoria-metrics-package-deb: victoria-metrics
	echo "GO_BUILDINFO:" "$(GO_BUILDINFO)"
	echo "PWD:" "$(PWD)"
	./package/package_deb.sh amd64

### Packaging for Debian - arm64
victoria-metrics-package-deb-arm64: victoria-metrics-arm64
	./package/package_deb.sh arm64

victoria-metrics-package-deb-all: \
        victoria-metrics-package-deb \
        victoria-metrics-package-deb-arm64

