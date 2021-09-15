# All these commands must run from repository root.

run-vmselect:
	mkdir -p vmselect-cache
	DOCKER_OPTS='-v $(shell pwd)/vmselect-cache:/cache' \
	APP_NAME=vmselect \
	ARGS='-storageNode=localhost -selectNode=localhost -cacheDataPath=/cache' \
	$(MAKE) run-via-docker

vmselect:
	APP_NAME=vmselect $(MAKE) app-local

vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) app-local

vmselect-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker

vmselect-pure-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-pure

vmselect-amd64-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-amd64

vmselect-arm-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-arm

vmselect-arm64-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-arm64

vmselect-ppc64le-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-ppc64le

vmselect-386-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-386

vmselect-prod-race:
	APP_NAME=vmselect RACE=-race $(MAKE) app-via-docker

package-vmselect:
	APP_NAME=vmselect $(MAKE) package-via-docker

package-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) package-via-docker

publish-vmselect:
	APP_NAME=vmselect $(MAKE) publish-via-docker

publish-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) publish-via-docker

vmselect-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmselect-local-with-goarch

vmselect-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmselect-local-with-goarch

vmselect-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmselect-local-with-goarch

vmselect-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmselect-local-with-goarch

vmselect-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmselect-local-with-goarch

vmselect-local-with-goarch:
	APP_NAME=vmselect $(MAKE) app-local-with-goarch

vmselect-pure:
	APP_NAME=vmselect $(MAKE) app-local-pure
