# All these commands must run from repository root.

vmalert:
	APP_NAME=vmalert $(MAKE) app-local

vmalert-race:
	APP_NAME=vmalert RACE=-race $(MAKE) app-local

vmalert-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker

vmalert-pure-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-pure

vmalert-linux-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-amd64

vmalert-linux-arm-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-arm

vmalert-linux-arm64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-arm64

vmalert-linux-ppc64le-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-ppc64le

vmalert-linux-386-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-386

vmalert-darwin-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-darwin-amd64

vmalert-darwin-arm64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-darwin-arm64

vmalert-freebsd-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-freebsd-amd64

vmalert-openbsd-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-openbsd-amd64

vmalert-windows-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-windows-amd64

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
	go test -v -race -cover ./app/vmalert/rule
	go test -v -race -cover ./app/vmalert/templates
	go test -v -race -cover ./app/vmalert/datasource
	go test -v -race -cover ./app/vmalert/notifier
	go test -v -race -cover ./app/vmalert/config
	go test -v -race -cover ./app/vmalert/remotewrite
	go test -v -race -cover ./app/vmalert/utils

run-vmalert: vmalert
	./bin/vmalert -rule=app/vmalert/config/testdata/rules/rules2-good.rules \
		-datasource.url=http://localhost:8428 \
		-notifier.url=http://localhost:9093 \
		-notifier.url=http://127.0.0.1:9093 \
		-remoteWrite.url=http://localhost:8428 \
		-remoteRead.url=http://localhost:8428 \
		-external.label=cluster=east-1 \
		-external.label=replica=a \
		-evaluationInterval=3s \
		-configCheckInterval=10s

run-vmalert-sd: vmalert
	./bin/vmalert -rule=app/vmalert/config/testdata/rules2-good.rules \
		-datasource.url=http://localhost:8428 \
		-remoteWrite.url=http://localhost:8428 \
		-notifier.config=app/vmalert/notifier/testdata/mixed.good.yaml \
		-configCheckInterval=10s

replay-vmalert: vmalert
	./bin/vmalert -rule=app/vmalert/config/testdata/rules/rules-replay-good.rules \
		-datasource.url=http://localhost:8428 \
		-remoteWrite.url=http://localhost:8428 \
		-external.label=cluster=east-1 \
		-external.label=replica=a \
		-replay.timeFrom=2024-06-01T00:00:00Z

vmalert-linux-amd64:
	APP_NAME=vmalert CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-linux-arm:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmalert-linux-arm64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmalert-linux-ppc64le:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmalert-linux-s390x:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmalert-linux-loong64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmalert-linux-386:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmalert-darwin-amd64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-darwin-arm64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmalert-freebsd-amd64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-openbsd-amd64:
	APP_NAME=vmalert CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-windows-amd64:
	GOARCH=amd64 APP_NAME=vmalert $(MAKE) app-local-windows-goarch

vmalert-pure:
	APP_NAME=vmalert $(MAKE) app-local-pure
