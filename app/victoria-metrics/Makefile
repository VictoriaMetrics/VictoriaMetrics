# All these commands must run from repository root.

victoria-metrics:
	APP_NAME=victoria-metrics $(MAKE) app-local

victoria-metrics-race:
	APP_NAME=victoria-metrics RACE=-race $(MAKE) app-local

victoria-metrics-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker

victoria-metrics-pure-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-pure

victoria-metrics-linux-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-linux-amd64

victoria-metrics-linux-arm-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-linux-arm

victoria-metrics-linux-arm64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-linux-arm64

victoria-metrics-linux-ppc64le-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-linux-ppc64le

victoria-metrics-linux-386-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-linux-386

victoria-metrics-darwin-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-darwin-amd64

victoria-metrics-darwin-arm64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-darwin-arm64

victoria-metrics-freebsd-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-freebsd-amd64

victoria-metrics-openbsd-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-openbsd-amd64

victoria-metrics-windows-amd64-prod:
	APP_NAME=victoria-metrics $(MAKE) app-via-docker-windows-amd64

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

victoria-metrics-linux-amd64:
	APP_NAME=victoria-metrics CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-metrics-linux-arm:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

victoria-metrics-linux-arm64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-metrics-linux-ppc64le:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

victoria-metrics-linux-s390x:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

victoria-metrics-linux-loong64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

victoria-metrics-linux-386:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

victoria-metrics-darwin-amd64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-metrics-darwin-arm64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-metrics-freebsd-amd64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-metrics-openbsd-amd64:
	APP_NAME=victoria-metrics CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-metrics-windows-amd64:
	GOARCH=amd64 APP_NAME=victoria-metrics $(MAKE) app-local-windows-goarch

victoria-metrics-pure:
	APP_NAME=victoria-metrics $(MAKE) app-local-pure

### Packaging as DEB - amd64
victoria-metrics-package-deb-amd64: victoria-metrics-linux-amd64-prod
	./package/package_deb.sh amd64

### Packaging as DEB - arm64
victoria-metrics-package-deb-arm: victoria-metrics-linux-arm-prod
	./package/package_deb.sh arm

### Packaging as DEB - arm64
victoria-metrics-package-deb-arm64: victoria-metrics-linux-arm64-prod
	./package/package_deb.sh arm64

### Packaging as DEB - all
victoria-metrics-package-deb: \
        victoria-metrics-package-deb-amd64 \
        victoria-metrics-package-deb-arm \
        victoria-metrics-package-deb-arm64

### Packaging as RPM - amd64
victoria-metrics-package-rpm-amd64: victoria-metrics-linux-amd64-prod
	./package/package_rpm.sh amd64

### Packaging as RPM - arm64
victoria-metrics-package-rpm-arm64: victoria-metrics-linux-arm64-prod
	./package/package_rpm.sh arm64

### Packaging as RPM - all
victoria-metrics-package-rpm: \
        victoria-metrics-package-rpm-amd64 \
        victoria-metrics-package-rpm-arm64

### Packaging as both DEB and RPM - all
victoria-metrics-package-deb-rpm: \
        victoria-metrics-package-deb \
        victoria-metrics-package-rpm
