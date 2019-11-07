# All these commands must run from repository root.

vmbackup:
	APP_NAME=vmbackup $(MAKE) app-local

vmbackup-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker

package-vmbackup:
	APP_NAME=vmbackup $(MAKE) package-via-docker

publish-vmbackup:
	APP_NAME=vmbackup $(MAKE) publish-via-docker

vmbackup-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-arm ./app/vmbackup

vmbackup-arm-prod:
	APP_NAME=vmbackup APP_SUFFIX='-arm' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm' $(MAKE) app-via-docker

vmbackup-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-arm64 ./app/vmbackup

vmbackup-arm64-prod:
	APP_NAME=vmbackup APP_SUFFIX='-arm64' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=arm64' $(MAKE) app-via-docker

vmbackup-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-386 ./app/vmbackup

vmbackup-386-prod:
	APP_NAME=vmbackup APP_SUFFIX='-386' DOCKER_OPTS='--env CGO_ENABLED=0 --env GOARCH=386' $(MAKE) app-via-docker

vmbackup-pure:
	APP_NAME=vmbackup $(MAKE) app-local-pure

vmbackup-pure-prod:
	APP_NAME=vmbackup APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker
