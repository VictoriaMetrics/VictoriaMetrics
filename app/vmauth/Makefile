# All these commands must run from repository root.

# Application build
vmauth:
	APP_NAME=vmauth $(MAKE) app-local

vmauth-race:
	APP_NAME=vmauth RACE=-race $(MAKE) app-local

vmauth-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker

vmauth-pure-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-pure

# GOARCH=amd64
vmauth-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-amd64

vmauth-linux-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-amd64

vmauth-darwin-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-darwin-amd64

vmauth-windows-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-windows-amd64

# GOARCH=arm64
vmauth-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-arm64

vmauth-linux-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-arm64

vmauth-darwin-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-darwin-arm64

# GOARCH=arm
vmauth-arm-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-arm

vmauth-linux-arm-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-arm

# GOARCH=ppc64le
vmauth-ppc64le-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-ppc64le

# GOARCH=386
vmauth-386-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-386

# Package build
package-vmauth:
	APP_NAME=vmauth $(MAKE) package-via-docker

package-vmauth-pure:
	APP_NAME=vmauth $(MAKE) package-via-docker-pure

package-vmauth-amd64:
	APP_NAME=vmauth $(MAKE) package-via-docker-amd64

package-vmauth-arm:
	APP_NAME=vmauth $(MAKE) package-via-docker-arm

package-vmauth-arm64:
	APP_NAME=vmauth $(MAKE) package-via-docker-arm64

package-vmauth-ppc64le:
	APP_NAME=vmauth $(MAKE) package-via-docker-ppc64le

package-vmauth-386:
	APP_NAME=vmauth $(MAKE) package-via-docker-386

publish-vmauth:
	APP_NAME=vmauth $(MAKE) publish-via-docker

run-vmauth:
	APP_NAME=vmauth \
	DOCKER_OPTS='-v $(shell pwd)/app/vmauth/:/app/vmauth' \
	ARGS='-auth.config=app/vmauth/example_config.yml' \
	$(MAKE) run-via-docker

vmauth-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmauth-local-with-goarch

vmauth-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmauth-local-with-goarch

vmauth-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmauth-local-with-goarch

vmauth-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmauth-local-with-goarch

vmauth-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmauth-local-with-goarch

vmauth-local-with-goarch:
	APP_NAME=vmauth $(MAKE) app-local-with-goarch

vmauth-pure:
	APP_NAME=vmauth $(MAKE) app-local-pure

vmauth-windows-amd64:
	GOARCH=amd64 APP_NAME=vmauth $(MAKE) app-local-windows-with-goarch
