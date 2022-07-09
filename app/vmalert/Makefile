# All these commands must run from repository root.

# Application build
vmalert:
	APP_NAME=vmalert $(MAKE) app-local

vmalert-race:
	APP_NAME=vmalert RACE=-race $(MAKE) app-local

vmalert-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker

vmalert-pure-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-pure

# GOOS=linux
vmalert-linux-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-amd64

vmalert-linux-arm64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-arm64

vmalert-linux-arm-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-linux-arm

# GOOS=darwin
vmalert-darwin-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-darwin-amd64

vmalert-darwin-arm64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-darwin-arm64

# GOOS=windows
vmalert-windows-amd64-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-windows-amd64

# GOARCH=ppc64le
vmalert-ppc64le-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-ppc64le

# GOARCH=386
vmalert-386-prod:
	APP_NAME=vmalert $(MAKE) app-via-docker-386

# Package build
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
	go test -v -race -cover ./app/vmalert/templates
	go test -v -race -cover ./app/vmalert/datasource
	go test -v -race -cover ./app/vmalert/notifier
	go test -v -race -cover ./app/vmalert/config
	go test -v -race -cover ./app/vmalert/remotewrite

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
		-replay.timeFrom=2021-05-11T07:21:43Z \
		-replay.timeTo=2021-05-29T18:40:43Z

vmalert-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmalert-local-with-goarch

vmalert-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmalert-local-with-goarch

vmalert-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmalert-local-with-goarch

vmalert-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmalert-local-with-goarch

vmalert-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmalert-local-with-goarch

vmalert-local-with-goarch:
	APP_NAME=vmalert $(MAKE) app-local-with-goarch

vmalert-pure:
	APP_NAME=vmalert $(MAKE) app-local-pure

vmalert-windows-amd64:
	GOARCH=amd64 APP_NAME=vmalert $(MAKE) app-local-windows-with-goarch
