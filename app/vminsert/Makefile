# All these commands must run from repository root.

run-vminsert:
	APP_NAME=vminsert ARGS='-storageNode=localhost' $(MAKE) run-via-docker

vminsert:
	APP_NAME=vminsert $(MAKE) app-local

vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) app-local

vminsert-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker

vminsert-pure-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-pure

vminsert-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-amd64

vminsert-arm-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-arm

vminsert-arm64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-arm64

vminsert-ppc64le-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-ppc64le

vminsert-386-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-386

vminsert-prod-race:
	APP_NAME=vminsert RACE=-race $(MAKE) app-via-docker

package-vminsert:
	APP_NAME=vminsert $(MAKE) package-via-docker

package-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) package-via-docker

publish-vminsert:
	APP_NAME=vminsert $(MAKE) publish-via-docker

publish-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) publish-via-docker

vminsert-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vminsert-local-with-goarch

vminsert-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vminsert-local-with-goarch

vminsert-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vminsert-local-with-goarch

vminsert-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vminsert-local-with-goarch

vminsert-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vminsert-local-with-goarch

vminsert-local-with-goarch:
	APP_NAME=vminsert $(MAKE) app-local-with-goarch

vminsert-pure:
	APP_NAME=vminsert $(MAKE) app-local-pure
