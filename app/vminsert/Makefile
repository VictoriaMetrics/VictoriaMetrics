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

vminsert-linux-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-linux-amd64

vminsert-linux-arm-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-linux-arm

vminsert-linux-arm64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-linux-arm64

vminsert-linux-ppc64le-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-linux-ppc64le

vminsert-linux-386-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-linux-386

vminsert-freebsd-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-freebsd-amd64

vminsert-openbsd-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-openbsd-amd64

vminsert-windows-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-windows-amd64

vminsert-darwin-amd64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-darwin-amd64

vminsert-darwin-arm64-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker-darwin-arm64

vminsert-prod-race:
	APP_NAME=vminsert RACE=-race $(MAKE) app-via-docker

package-vminsert:
	APP_NAME=vminsert $(MAKE) package-via-docker

package-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) package-via-docker

package-vminsert-amd64:
	APP_NAME=vminsert $(MAKE) package-via-docker-amd64

package-vminsert-arm:
	APP_NAME=vminsert $(MAKE) package-via-docker-arm

package-vminsert-arm64:
	APP_NAME=vminsert $(MAKE) package-via-docker-arm64

package-vminsert-ppc64le:
	APP_NAME=vminsert $(MAKE) package-via-docker-ppc64le

package-vminsert-386:
	APP_NAME=vminsert $(MAKE) package-via-docker-386

publish-vminsert:
	APP_NAME=vminsert $(MAKE) publish-via-docker

publish-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) publish-via-docker

vminsert-linux-amd64:
	APP_NAME=vminsert CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vminsert-linux-arm:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vminsert-linux-arm64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vminsert-linux-ppc64le:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vminsert-linux-s390x:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vminsert-linux-loong64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vminsert-linux-386:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vminsert-freebsd-amd64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vminsert-openbsd-amd64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vminsert-windows-amd64:
	GOARCH=amd64 APP_NAME=vminsert $(MAKE) app-local-windows-goarch

vminsert-darwin-amd64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vminsert-darwin-arm64:
	APP_NAME=vminsert CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vminsert-pure:
	APP_NAME=vminsert $(MAKE) app-local-pure
