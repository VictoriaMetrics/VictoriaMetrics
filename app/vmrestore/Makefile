# All these commands must run from repository root.

vmrestore:
	APP_NAME=vmrestore $(MAKE) app-local

vmrestore-race:
	APP_NAME=vmrestore RACE=-race $(MAKE) app-local

vmrestore-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker

vmrestore-pure-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-pure

vmrestore-amd64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-amd64

vmrestore-arm-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm

vmrestore-arm64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm64

vmrestore-ppc64le-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-ppc64le

vmrestore-386-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-386

package-vmrestore:
	APP_NAME=vmrestore $(MAKE) package-via-docker

package-vmrestore-pure:
	APP_NAME=vmrestore $(MAKE) package-via-docker-pure

package-vmrestore-amd64:
	APP_NAME=vmrestore $(MAKE) package-via-docker-amd64

package-vmrestore-arm:
	APP_NAME=vmrestore $(MAKE) package-via-docker-arm

package-vmrestore-arm64:
	APP_NAME=vmrestore $(MAKE) package-via-docker-arm64

package-vmrestore-ppc64le:
	APP_NAME=vmrestore $(MAKE) package-via-docker-ppc64le

package-vmrestore-386:
	APP_NAME=vmrestore $(MAKE) package-via-docker-386

publish-vmrestore:
	APP_NAME=vmrestore $(MAKE) publish-via-docker

vmrestore-pure:
	APP_NAME=vmrestore $(MAKE) app-local-pure

vmrestore-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-amd64 ./app/vmrestore

vmrestore-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-arm ./app/vmrestore

vmrestore-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-arm64 ./app/vmrestore

vmrestore-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-ppc64le ./app/vmrestore

vmrestore-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmrestore-386 ./app/vmrestore
