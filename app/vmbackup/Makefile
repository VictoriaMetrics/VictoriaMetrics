# All these commands must run from repository root.

vmbackup:
	APP_NAME=vmbackup $(MAKE) app-local

vmbackup-race:
	APP_NAME=vmbackup RACE=-race $(MAKE) app-local

vmbackup-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker

vmbackup-pure-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-pure

vmbackup-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-amd64

vmbackup-arm-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-arm

vmbackup-arm64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-arm64

vmbackup-ppc64le-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-ppc64le

vmbackup-386-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-386

package-vmbackup:
	APP_NAME=vmbackup $(MAKE) package-via-docker

package-vmbackup-pure:
	APP_NAME=vmbackup $(MAKE) package-via-docker-pure

package-vmbackup-amd64:
	APP_NAME=vmbackup $(MAKE) package-via-docker-amd64

package-vmbackup-arm:
	APP_NAME=vmbackup $(MAKE) package-via-docker-arm

package-vmbackup-arm64:
	APP_NAME=vmbackup $(MAKE) package-via-docker-arm64

package-vmbackup-ppc64le:
	APP_NAME=vmbackup $(MAKE) package-via-docker-ppc64le

package-vmbackup-386:
	APP_NAME=vmbackup $(MAKE) package-via-docker-386

publish-vmbackup:
	APP_NAME=vmbackup $(MAKE) publish-via-docker

vmbackup-pure:
	APP_NAME=vmbackup $(MAKE) app-local-pure

vmbackup-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-amd64 ./app/vmbackup

vmbackup-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-arm ./app/vmbackup

vmbackup-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-arm64 ./app/vmbackup

vmbackup-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-ppc64le ./app/vmbackup

vmbackup-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmbackup-386 ./app/vmbackup
