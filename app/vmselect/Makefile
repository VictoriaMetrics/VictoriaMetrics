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

vmselect-linux-amd64-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-linux-amd64

vmselect-linux-arm-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-linux-arm

vmselect-linux-arm64-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-linux-arm64

vmselect-linux-ppc64le-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-linux-ppc64le

vmselect-linux-386-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-linux-386

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

vmselect-linux-amd64:
	APP_NAME=vmselect CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmselect-linux-arm:
	APP_NAME=vmselect CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmselect-linux-arm64:
	APP_NAME=vmselect CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmselect-linux-ppc64le:
	APP_NAME=vmselect CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmselect-linux-386:
	APP_NAME=vmselect CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmselect-pure:
	APP_NAME=vmselect $(MAKE) app-local-pure
