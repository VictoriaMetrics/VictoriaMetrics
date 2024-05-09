# All these commands must run from repository root.

vmagent:
	APP_NAME=vmagent $(MAKE) app-local

vmagent-race:
	APP_NAME=vmagent RACE=-race $(MAKE) app-local

vmagent-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker

vmagent-pure-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-pure

vmagent-linux-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-amd64

vmagent-linux-arm-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-arm

vmagent-linux-arm64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-arm64

vmagent-linux-ppc64le-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-ppc64le

vmagent-linux-386-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-386

vmagent-darwin-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-darwin-amd64

vmagent-darwin-arm64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-darwin-arm64

vmagent-freebsd-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-freebsd-amd64

vmagent-openbsd-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-openbsd-amd64

vmagent-windows-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-windows-amd64

package-vmagent:
	APP_NAME=vmagent $(MAKE) package-via-docker

package-vmagent-pure:
	APP_NAME=vmagent $(MAKE) package-via-docker-pure

package-vmagent-amd64:
	APP_NAME=vmagent $(MAKE) package-via-docker-amd64

package-vmagent-arm:
	APP_NAME=vmagent $(MAKE) package-via-docker-arm

package-vmagent-arm64:
	APP_NAME=vmagent $(MAKE) package-via-docker-arm64

package-vmagent-ppc64le:
	APP_NAME=vmagent $(MAKE) package-via-docker-ppc64le

package-vmagent-386:
	APP_NAME=vmagent $(MAKE) package-via-docker-386

publish-vmagent:
	APP_NAME=vmagent $(MAKE) publish-via-docker

run-vmagent:
	mkdir -p vmagent-remotewrite-data
	DOCKER_OPTS='-v $(shell pwd)/vmagent-remotewrite-data:/vmagent-remotewrite-data' \
	ARGS='-remoteWrite.url=http://localhost:8428/api/v1/write' \
	APP_NAME=vmagent \
	$(MAKE) run-via-docker

vmagent-linux-amd64:
	APP_NAME=vmagent CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmagent-linux-arm:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmagent-linux-arm64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmagent-linux-ppc64le:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmagent-linux-s390x:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmagent-linux-loong64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmagent-linux-386:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmagent-darwin-amd64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmagent-darwin-arm64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmagent-freebsd-amd64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmagent-openbsd-amd64:
	APP_NAME=vmagent CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmagent-windows-amd64:
	GOARCH=amd64 APP_NAME=vmagent $(MAKE) app-local-windows-goarch

vmagent-pure:
	APP_NAME=vmagent $(MAKE) app-local-pure
