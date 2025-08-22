# All these commands must run from repository root.

# special tag to reduce resulting binary size
# See this issue https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8008
VMRESTORE_GO_BUILD_TAGS=disable_grpc_modules

vmrestore:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-local

vmrestore-race:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) RACE=-race $(MAKE) app-local

vmrestore-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker

vmrestore-pure-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-pure

vmrestore-linux-amd64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-linux-amd64

vmrestore-linux-arm-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-linux-arm

vmrestore-linux-arm64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-linux-arm64

vmrestore-linux-ppc64le-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-linux-ppc64le

vmrestore-linux-386-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-linux-386

vmrestore-darwin-amd64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-darwin-amd64

vmrestore-darwin-arm64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-darwin-arm64

vmrestore-freebsd-amd64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-freebsd-amd64

vmrestore-openbsd-amd64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-openbsd-amd64

vmrestore-windows-amd64-prod:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-via-docker-windows-amd64

package-vmrestore:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker

package-vmrestore-pure:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-pure

package-vmrestore-amd64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-amd64

package-vmrestore-arm:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-arm

package-vmrestore-arm64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-arm64

package-vmrestore-ppc64le:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-ppc64le

package-vmrestore-386:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) package-via-docker-386

publish-vmrestore:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) publish-via-docker

vmrestore-linux-amd64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmrestore-linux-arm:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmrestore-linux-arm64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmrestore-linux-ppc64le:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmrestore-linux-s390x:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmrestore-linux-loong64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmrestore-linux-386:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmrestore-darwin-amd64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmrestore-darwin-arm64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmrestore-freebsd-amd64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmrestore-openbsd-amd64:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmrestore-windows-amd64:
	GOARCH=amd64 APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-local-windows-goarch

vmrestore-pure:
	APP_NAME=vmrestore EXTRA_GO_BUILD_TAGS=$(VMRESTORE_GO_BUILD_TAGS) $(MAKE) app-local-pure
