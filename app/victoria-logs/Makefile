# All these commands must run from repository root.

victoria-logs:
	APP_NAME=victoria-logs $(MAKE) app-local

victoria-logs-race:
	APP_NAME=victoria-logs RACE=-race $(MAKE) app-local

victoria-logs-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker

victoria-logs-pure-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-pure

victoria-logs-linux-amd64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-linux-amd64

victoria-logs-linux-arm-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-linux-arm

victoria-logs-linux-arm64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-linux-arm64

victoria-logs-linux-ppc64le-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-linux-ppc64le

victoria-logs-linux-386-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-linux-386

victoria-logs-darwin-amd64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-darwin-amd64

victoria-logs-darwin-arm64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-darwin-arm64

victoria-logs-freebsd-amd64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-freebsd-amd64

victoria-logs-openbsd-amd64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-openbsd-amd64

victoria-logs-windows-amd64-prod:
	APP_NAME=victoria-logs $(MAKE) app-via-docker-windows-amd64

package-victoria-logs:
	APP_NAME=victoria-logs $(MAKE) package-via-docker

package-victoria-logs-pure:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-pure

package-victoria-logs-amd64:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-amd64

package-victoria-logs-arm:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-arm

package-victoria-logs-arm64:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-arm64

package-victoria-logs-ppc64le:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-ppc64le

package-victoria-logs-386:
	APP_NAME=victoria-logs $(MAKE) package-via-docker-386

publish-victoria-logs:
	APP_NAME=victoria-logs $(MAKE) publish-via-docker

victoria-logs-linux-amd64:
	APP_NAME=victoria-logs CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-logs-linux-arm:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

victoria-logs-linux-arm64:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-logs-linux-ppc64le:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

victoria-logs-linux-s390x:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

victoria-logs-linux-386:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

victoria-logs-darwin-amd64:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-logs-darwin-arm64:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

victoria-logs-freebsd-amd64:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-logs-openbsd-amd64:
	APP_NAME=victoria-logs CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

victoria-logs-windows-amd64:
	GOARCH=amd64 APP_NAME=victoria-logs $(MAKE) app-local-windows-goarch

victoria-logs-pure:
	APP_NAME=victoria-logs $(MAKE) app-local-pure

run-victoria-logs:
	mkdir -p victoria-logs-data
	DOCKER_OPTS='-v $(shell pwd)/victoria-logs-data:/victoria-logs-data' \
	APP_NAME=victoria-logs \
	ARGS='' \
	$(MAKE) run-via-docker
