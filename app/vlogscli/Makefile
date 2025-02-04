# All these commands must run from repository root.

vlogscli:
	APP_NAME=vlogscli $(MAKE) app-local

vlogscli-race:
	APP_NAME=vlogscli RACE=-race $(MAKE) app-local

vlogscli-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker

vlogscli-pure-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-pure

vlogscli-linux-amd64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-linux-amd64

vlogscli-linux-arm-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-linux-arm

vlogscli-linux-arm64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-linux-arm64

vlogscli-linux-ppc64le-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-linux-ppc64le

vlogscli-linux-386-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-linux-386

vlogscli-darwin-amd64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-darwin-amd64

vlogscli-darwin-arm64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-darwin-arm64

vlogscli-freebsd-amd64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-freebsd-amd64

vlogscli-openbsd-amd64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-openbsd-amd64

vlogscli-windows-amd64-prod:
	APP_NAME=vlogscli $(MAKE) app-via-docker-windows-amd64

package-vlogscli:
	APP_NAME=vlogscli $(MAKE) package-via-docker

package-vlogscli-pure:
	APP_NAME=vlogscli $(MAKE) package-via-docker-pure

package-vlogscli-amd64:
	APP_NAME=vlogscli $(MAKE) package-via-docker-amd64

package-vlogscli-arm:
	APP_NAME=vlogscli $(MAKE) package-via-docker-arm

package-vlogscli-arm64:
	APP_NAME=vlogscli $(MAKE) package-via-docker-arm64

package-vlogscli-ppc64le:
	APP_NAME=vlogscli $(MAKE) package-via-docker-ppc64le

package-vlogscli-386:
	APP_NAME=vlogscli $(MAKE) package-via-docker-386

publish-vlogscli:
	APP_NAME=vlogscli $(MAKE) publish-via-docker

vlogscli-linux-amd64:
	APP_NAME=vlogscli CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlogscli-linux-arm:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vlogscli-linux-arm64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vlogscli-linux-ppc64le:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vlogscli-linux-s390x:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vlogscli-linux-loong64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vlogscli-linux-386:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vlogscli-darwin-amd64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlogscli-darwin-arm64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vlogscli-freebsd-amd64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlogscli-openbsd-amd64:
	APP_NAME=vlogscli CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlogscli-windows-amd64:
	GOARCH=amd64 APP_NAME=vlogscli $(MAKE) app-local-windows-goarch

vlogscli-pure:
	APP_NAME=vlogscli $(MAKE) app-local-pure

run-vlogscli:
	APP_NAME=vlogscli $(MAKE) run-via-docker
