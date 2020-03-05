# All these commands must run from repository root.

victoria-metrics:
	APP_NAME=victoria-metrics $(MAKE) app-local

victoria-metrics-race:
	APP_NAME=victoria-metrics RACE=-race $(MAKE) app-local

victoria-metrics-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker

victoria-metrics-pure-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-pure

victoria-metrics-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-amd64

victoria-metrics-arm-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-arm

victoria-metrics-arm64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-arm64

victoria-metrics-ppc64le-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-ppc64le

victoria-metrics-386-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-386

package-victoria-metrics:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker

package-victoria-metrics-pure:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-pure

package-victoria-metrics-amd64:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-amd64

package-victoria-metrics-arm:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-arm

package-victoria-metrics-arm64:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-arm64

package-victoria-metrics-ppc64le:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-ppc64le

package-victoria-metrics-386:
	APP_NAME=victoria-metrics $(MAKE) package-via-docker-386

publish-victoria-metrics:
	APP_NAME=victoria-metrics $(MAKE) publish-via-docker

run-victoria-metrics:
	mkdir -p victoria-metrics-data
	DOCKER_OPTS='-v $(shell pwd)/victoria-metrics-data:/victoria-metrics-data' \
	APP_NAME=victoria-metrics \
	ARGS='-graphiteListenAddr=:2003 -opentsdbListenAddr=:4242 -retentionPeriod=12 -search.maxUniqueTimeseries=1000000 -search.maxQueryDuration=10m' \
	$(MAKE) run-via-docker

victoria-metrics-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-amd64 ./app/victoria-metrics

victoria-metrics-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-arm ./app/victoria-metrics

victoria-metrics-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-arm64 ./app/victoria-metrics

victoria-metrics-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-ppc64le ./app/victoria-metrics

victoria-metrics-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/victoria-metrics-386 ./app/victoria-metrics

victoria-metrics-pure:
	APP_NAME=victoria-metrics $(MAKE) app-local-pure

### Packaging as DEB - amd64
victoria-metrics-package-deb: victoria-metrics-prod
	./package/package_deb.sh amd64

### Packaging as DEB - arm64
victoria-metrics-package-deb-arm64: victoria-metrics-arm64-prod
	./package/package_deb.sh arm64

### Packaging as DEB - all
victoria-metrics-package-deb-all: \
        victoria-metrics-package-deb \
        victoria-metrics-package-deb-arm64

### Packaging as RPM - amd64
victoria-metrics-package-rpm: victoria-metrics-prod
	./package/package_rpm.sh amd64

### Packaging as RPM - arm64
victoria-metrics-package-rpm-arm64: victoria-metrics-arm64-prod
	./package/package_rpm.sh arm64

### Packaging as RPM - all
victoria-metrics-package-rpm-all: \
        victoria-metrics-package-rpm \
        victoria-metrics-package-rpm-arm64

### Packaging as both DEB and RPM - all
victoria-metrics-package-deb-rpm-all: \
        victoria-metrics-package-deb \
        victoria-metrics-package-deb-arm64 \
        victoria-metrics-package-rpm \
        victoria-metrics-package-rpm-arm64
