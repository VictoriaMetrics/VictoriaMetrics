# All these commands must run from repository root.

vmalert-tool:
	APP_NAME=vmalert-tool $(MAKE) app-local

vmalert-tool-race:
	APP_NAME=vmalert-tool RACE=-race $(MAKE) app-local

vmalert-tool-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker

vmalert-tool-pure-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-pure

vmalert-tool-linux-amd64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-linux-amd64

vmalert-tool-linux-arm-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-linux-arm

vmalert-tool-linux-arm64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-linux-arm64

vmalert-tool-linux-ppc64le-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-linux-ppc64le

vmalert-tool-linux-386-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-linux-386

vmalert-tool-darwin-amd64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-darwin-amd64

vmalert-tool-darwin-arm64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-darwin-arm64

vmalert-tool-freebsd-amd64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-freebsd-amd64

vmalert-tool-openbsd-amd64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-openbsd-amd64

vmalert-tool-windows-amd64-prod:
	APP_NAME=vmalert-tool $(MAKE) app-via-docker-windows-amd64

package-vmalert-tool:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker

package-vmalert-tool-pure:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-pure

package-vmalert-tool-amd64:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-amd64

package-vmalert-tool-arm:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-arm

package-vmalert-tool-arm64:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-arm64

package-vmalert-tool-ppc64le:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-ppc64le

package-vmalert-tool-386:
	APP_NAME=vmalert-tool $(MAKE) package-via-docker-386

publish-vmalert-tool:
	APP_NAME=vmalert-tool $(MAKE) publish-via-docker

vmalert-tool-linux-amd64:
	APP_NAME=vmalert-tool CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-tool-linux-arm:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmalert-tool-linux-arm64:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmalert-tool-linux-ppc64le:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmalert-tool-linux-s390x:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmalert-tool-linux-386:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmalert-tool-darwin-amd64:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-tool-darwin-arm64:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmalert-tool-freebsd-amd64:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-tool-openbsd-amd64:
	APP_NAME=vmalert-tool CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmalert-tool-windows-amd64:
	GOARCH=amd64 APP_NAME=vmalert-tool $(MAKE) app-local-windows-goarch

vmalert-tool-pure:
	APP_NAME=vmalert-tool $(MAKE) app-local-pure
