# All these commands must run from repository root.

run-vmstorage:
	mkdir -p vmstorage-data
	DOCKER_OPTS='-v $(shell pwd)/vmstorage-data:/vmstorage-data' \
	APP_NAME=vmstorage \
	ARGS='-retentionPeriod=12' \
	$(MAKE) run-via-docker

vmstorage:
	APP_NAME=vmstorage $(MAKE) app-local

vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) app-local

vmstorage-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker

vmstorage-pure-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-pure

vmstorage-linux-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-linux-amd64

vmstorage-linux-arm-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-linux-arm

vmstorage-linux-arm64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-linux-arm64

vmstorage-linux-ppc64le-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-linux-ppc64le

vmstorage-linux-386-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-linux-386

vmstorage-freebsd-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-freebsd-amd64

vmstorage-openbsd-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-openbsd-amd64

vmstorage-windows-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-windows-amd64

vmstorage-darwin-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-darwin-amd64

vmstorage-darwin-arm64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-darwin-arm64

vmstorage-prod-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) app-via-docker

package-vmstorage:
	APP_NAME=vmstorage $(MAKE) package-via-docker

package-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) package-via-docker

package-vmstorage-amd64:
	APP_NAME=vmstorage $(MAKE) package-via-docker-amd64

package-vmstorage-arm:
	APP_NAME=vmstorage $(MAKE) package-via-docker-arm

package-vmstorage-arm64:
	APP_NAME=vmstorage $(MAKE) package-via-docker-arm64

package-vmstorage-ppc64le:
	APP_NAME=vmstorage $(MAKE) package-via-docker-ppc64le

package-vmstorage-386:
	APP_NAME=vmstorage $(MAKE) package-via-docker-386

publish-vmstorage:
	APP_NAME=vmstorage $(MAKE) publish-via-docker

publish-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) publish-via-docker

vmstorage-linux-amd64:
	APP_NAME=vmstorage CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmstorage-linux-arm:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmstorage-linux-arm64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmstorage-linux-ppc64le:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmstorage-linux-s390x:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmstorage-linux-loong64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmstorage-linux-386:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmstorage-freebsd-amd64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmstorage-openbsd-amd64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmstorage-windows-amd64:
	GOARCH=amd64 APP_NAME=vmstorage $(MAKE) app-local-windows-goarch

vmstorage-darwin-amd64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmstorage-darwin-arm64:
	APP_NAME=vmstorage CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmstorage-pure:
	APP_NAME=vmstorage $(MAKE) app-local-pure
