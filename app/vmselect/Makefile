# All these commands must run from repository root.

run-vmselect:
	mkdir -p vmselect-cache
	DOCKER_OPTS='-v $(shell pwd)/vmselect-cache:/cache' \
	APP_NAME=vmselect \
	ARGS='-storageNode=localhost:8401 -selectNode=localhost:8481 -cacheDataPath=/cache' \
	$(MAKE) run-via-docker

vmselect:
	APP_NAME=vmselect $(MAKE) vmui app-local

vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) vmui app-local

vmselect-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker

vmselect-pure-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-pure

vmselect-amd64-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-amd64

vmselect-arm-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-arm

vmselect-arm64-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-arm64

vmselect-ppc64le-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-ppc64le

vmselect-386-prod:
	APP_NAME=vmselect $(MAKE) vmui app-via-docker-386

vmselect-prod-race:
	APP_NAME=vmselect RACE=-race $(MAKE) vmui app-via-docker

package-vmselect:
	APP_NAME=vmselect $(MAKE) vmui package-via-docker

package-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) vmui package-via-docker

publish-vmselect:
	APP_NAME=vmselect $(MAKE) vmui publish-via-docker

publish-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) vmui publish-via-docker

vmselect-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmui vmselect-local-with-goarch

vmselect-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmui vmselect-local-with-goarch

vmselect-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmui vmselect-local-with-goarch

vmselect-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmui vmselect-local-with-goarch

vmselect-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmui vmselect-local-with-goarch

vmselect-local-with-goarch:
	APP_NAME=vmselect $(MAKE) vmui app-local-with-goarch

vmselect-pure:
	APP_NAME=vmselect $(MAKE) vmui app-local-pure
