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

vmstorage-amd64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-amd64

vmstorage-arm-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-arm

vmstorage-arm64-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-arm64

vmstorage-ppc64le-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-ppc64le

vmstorage-386-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker-386

vmstorage-prod-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) app-via-docker

package-vmstorage:
	APP_NAME=vmstorage $(MAKE) package-via-docker

package-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) package-via-docker

publish-vmstorage:
	APP_NAME=vmstorage $(MAKE) publish-via-docker

publish-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) publish-via-docker

vmstorage-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmstorage-local-with-goarch

vmstorage-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmstorage-local-with-goarch

vmstorage-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmstorage-local-with-goarch

vmstorage-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmstorage-local-with-goarch

vmstorage-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmstorage-local-with-goarch

vmstorage-local-with-goarch:
	APP_NAME=vmstorage $(MAKE) app-local-with-goarch

vmstorage-pure:
	APP_NAME=vmstorage $(MAKE) app-local-pure
