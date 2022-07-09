# All these commands must run from repository root.

# Application build
vmagent:
	APP_NAME=vmagent $(MAKE) app-local

vmagent-race:
	APP_NAME=vmagent RACE=-race $(MAKE) app-local

vmagent-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker

vmagent-pure-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-pure

# GOOS=linux
vmagent-linux-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-amd64

vmagent-linux-arm64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-arm64

vmagent-linux-arm-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-linux-arm

#GOOS=darwin
vmagent-darwin-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-darwin-amd64

vmagent-darwin-arm64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-darwin-arm64

#GOOS=windows
vmagent-windows-amd64-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-windows-amd64

# GOARCH=ppc64le
vmagent-ppc64le-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-ppc64le

# GOARCH=386
vmagent-386-prod:
	APP_NAME=vmagent $(MAKE) app-via-docker-386

# Package build
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

vmagent-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmagent-local-with-goarch

vmagent-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmagent-local-with-goarch

vmagent-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmagent-local-with-goarch

vmagent-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmagent-local-with-goarch

vmagent-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmagent-local-with-goarch

vmagent-local-with-goarch:
	APP_NAME=vmagent $(MAKE) app-local-with-goarch

vmagent-pure:
	APP_NAME=vmagent $(MAKE) app-local-pure

vmagent-windows-amd64:
	GOARCH=amd64 APP_NAME=vmagent $(MAKE) app-local-windows-with-goarch
