# All these commands must run from repository root.

vmrestore:
	APP_NAME=vmrestore $(MAKE) app-local

vmrestore-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker

package-vmrestore:
	APP_NAME=vmrestore $(MAKE) package-via-docker

publish-vmrestore:
	APP_NAME=vmrestore $(MAKE) publish-via-docker

vmrestore-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-arm ./app/vmrestore

vmrestore-arm-prod:
	APP_NAME=vmrestore APP_SUFFIX='-arm' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm' $(MAKE) app-via-docker

vmrestore-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-arm64 ./app/vmrestore

vmrestore-arm64-prod:
	APP_NAME=vmrestore APP_SUFFIX='-arm64' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm64' $(MAKE) app-via-docker

vmrestore-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-386 ./app/vmrestore

vmrestore-386-prod:
	APP_NAME=vmrestore APP_SUFFIX='-386' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=386' $(MAKE) app-via-docker

vmrestore-pure:
	APP_NAME=vmrestore $(MAKE) app-local-pure

vmrestore-pure-prod:
	APP_NAME=vmrestore APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker
