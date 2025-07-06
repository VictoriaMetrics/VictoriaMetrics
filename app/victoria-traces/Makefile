# All these commands must run from repository root.

victoria-traces:
	APP_NAME=victoria-traces $(MAKE) app-local

victoria-traces-race:
	APP_NAME=victoria-traces RACE=-race $(MAKE) app-local

victoria-traces-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker

victoria-traces-pure-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-pure

victoria-traces-linux-amd64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-linux-amd64

victoria-traces-linux-arm-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-linux-arm

victoria-traces-linux-arm64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-linux-arm64

victoria-traces-linux-ppc64le-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-linux-ppc64le

victoria-traces-linux-386-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-linux-386

victoria-traces-darwin-amd64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-darwin-amd64

victoria-traces-darwin-arm64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-darwin-arm64

victoria-traces-freebsd-amd64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-freebsd-amd64

victoria-traces-openbsd-amd64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-openbsd-amd64

victoria-traces-windows-amd64-prod:
	APP_NAME=victoria-traces $(MAKE) app-via-docker-windows-amd64

package-victoria-traces:
	APP_NAME=victoria-traces $(MAKE) package-via-docker

package-victoria-traces-pure:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-pure

package-victoria-traces-amd64:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-amd64

package-victoria-traces-arm:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-arm

package-victoria-traces-arm64:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-arm64

package-victoria-traces-ppc64le:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-ppc64le

package-victoria-traces-386:
	APP_NAME=victoria-traces $(MAKE) package-via-docker-386

publish-victoria-traces:
	APP_NAME=victoria-traces $(MAKE) publish-via-docker

victoria-traces-linux-amd64:
	APP_NAME=victoria-traces CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-traces-linux-arm:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

victoria-traces-linux-arm64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-traces-linux-ppc64le:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

victoria-traces-linux-s390x:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

victoria-traces-linux-loong64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

victoria-traces-linux-386:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

victoria-traces-darwin-amd64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-traces-darwin-arm64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-traces-freebsd-amd64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-traces-openbsd-amd64:
	APP_NAME=victoria-traces CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-traces-windows-amd64:
	GOARCH=amd64 APP_NAME=victoria-traces $(MAKE) app-local-windows-goarch

victoria-traces-pure:
	APP_NAME=victoria-traces $(MAKE) app-local-pure

run-victoria-traces:
	mkdir -p victoria-traces-data
	DOCKER_OPTS='-v $(shell pwd)/victoria-traces-data:/victoria-traces-data' \
	APP_NAME=victoria-traces \
	ARGS='' \
	$(MAKE) run-via-docker
