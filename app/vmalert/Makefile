# All these commands must run from repository root.

vmalert:
	APP_NAME=vmalert $(MAKE) app-local

vmalert-race:
	APP_NAME=vmalert RACE=-race $(MAKE) app-local

vmalert-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker

vmalert-pure-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-pure

vmalert-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-amd64

vmalert-arm-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-arm

vmalert-arm64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-arm64

vmalert-ppc64le-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-ppc64le

vmalert-386-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-386

package-vmalert:
	APP_NAME=vmalert $(MAKE) package-via-docker

package-vmalert-pure:
	APP_NAME=vmalert $(MAKE) package-via-docker-pure

package-vmalert-amd64:
	APP_NAME=vmalert $(MAKE) package-via-docker-amd64

package-vmalert-arm:
	APP_NAME=vmalert $(MAKE) package-via-docker-arm

package-vmalert-arm64:
	APP_NAME=vmalert $(MAKE) package-via-docker-arm64

package-vmalert-ppc64le:
	APP_NAME=vmalert $(MAKE) package-via-docker-ppc64le

package-vmalert-386:
	APP_NAME=vmalert $(MAKE) package-via-docker-386

publish-vmalert:
	APP_NAME=vmalert $(MAKE) publish-via-docker

test-vmalert:
	go test -v -race -cover ./app/vmalert -loggerLevel=ERROR
	go test -v -race -cover ./app/vmalert/datasource
	go test -v -race -cover ./app/vmalert/notifier
	go test -v -race -cover ./app/vmalert/config

run-vmalert: vmalert
	./bin/vmalert -rule=app/vmalert/config/testdata/rules2-good.rules \
		-datasource.url=http://localhost:8428 \
		-notifier.url=http://localhost:9093 \
		-notifier.url=http://127.0.0.1:9093 \
		-remoteWrite.url=http://localhost:8428 \
		-remoteRead.url=http://localhost:8428 \
		-evaluationInterval=3s

vmalert-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmalert-amd64 ./app/vmalert

vmalert-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmalert-arm ./app/vmalert

vmalert-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmalert-arm64 ./app/vmalert

vmalert-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmalert-ppc64le ./app/vmalert

vmalert-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 GO111MODULE=on go build -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/vmalert-386 ./app/vmalert

vmalert-pure:
	APP_NAME=vmalert $(MAKE) app-local-pure
