# All these commands must run from repository root.

vmauth:
	APP_NAME=vmauth $(MAKE) app-local

vmauth-race:
	APP_NAME=vmauth RACE=-race $(MAKE) app-local

vmauth-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker

vmauth-pure-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-pure

vmauth-amd64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-amd64

vmauth-arm-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-arm

vmauth-arm64-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-arm64

vmauth-ppc64le-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-ppc64le

vmauth-386-prod:
	APP_NAME=vmauth $(MAKE) app-via-docker-386

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
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmauth-amd64 ./app/vmauth

vmauth-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmauth-arm ./app/vmauth

vmauth-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmauth-arm64 ./app/vmauth

vmauth-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmauth-ppc64le ./app/vmauth

vmauth-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmauth-386 ./app/vmauth

vmauth-pure:
	APP_NAME=vmauth $(MAKE) app-local-pure
