# All these commands must run from repository root.

vmauth:
	APP_NAME=vmauth $(MAKE) app-local

vmauth-race:
	APP_NAME=vmauth RACE=-race $(MAKE) app-local

vmauth-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker

vmauth-pure-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-pure

vmauth-linux-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-amd64

vmauth-linux-arm-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-arm

vmauth-linux-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-arm64

vmauth-linux-ppc64le-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-ppc64le

vmauth-linux-386-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-linux-386

vmauth-darwin-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-darwin-amd64

vmauth-darwin-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-darwin-arm64

vmauth-freebsd-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-freebsd-amd64

vmauth-openbsd-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-openbsd-amd64

vmauth-windows-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-windows-amd64

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

vmauth-linux-amd64:
	APP_NAME=vmauth CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmauth-linux-arm:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmauth-linux-arm64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmauth-linux-ppc64le:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmauth-linux-s390x:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmauth-linux-loong64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmauth-linux-386:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmauth-darwin-amd64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmauth-darwin-arm64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmauth-freebsd-amd64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmauth-openbsd-amd64:
	APP_NAME=vmauth CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmauth-windows-amd64:
	GOARCH=amd64 APP_NAME=vmauth $(MAKE) app-local-windows-goarch

vmauth-pure:
	APP_NAME=vmauth $(MAKE) app-local-pure
