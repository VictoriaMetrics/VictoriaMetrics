# All these commands must run from repository root.

vlagent:
	APP_NAME=vlagent $(MAKE) app-local

vlagent-race:
	APP_NAME=vlagent RACE=-race $(MAKE) app-local

vlagent-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker

vlagent-pure-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-pure

vlagent-linux-amd64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-linux-amd64

vlagent-linux-arm-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-linux-arm

vlagent-linux-arm64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-linux-arm64

vlagent-linux-ppc64le-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-linux-ppc64le

vlagent-linux-386-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-linux-386

vlagent-darwin-amd64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-darwin-amd64

vlagent-darwin-arm64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-darwin-arm64

vlagent-freebsd-amd64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-freebsd-amd64

vlagent-openbsd-amd64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-openbsd-amd64

vlagent-windows-amd64-prod:
	APP_NAME=vlagent $(MAKE) app-via-docker-windows-amd64

package-vlagent:
	APP_NAME=vlagent $(MAKE) package-via-docker

package-vlagent-pure:
	APP_NAME=vlagent $(MAKE) package-via-docker-pure

package-vlagent-amd64:
	APP_NAME=vlagent $(MAKE) package-via-docker-amd64

package-vlagent-arm:
	APP_NAME=vlagent $(MAKE) package-via-docker-arm

package-vlagent-arm64:
	APP_NAME=vlagent $(MAKE) package-via-docker-arm64

package-vlagent-ppc64le:
	APP_NAME=vlagent $(MAKE) package-via-docker-ppc64le

package-vlagent-386:
	APP_NAME=vlagent $(MAKE) package-via-docker-386

publish-vlagent:
	APP_NAME=vlagent $(MAKE) publish-via-docker

vlagent-linux-amd64:
	APP_NAME=vlagent CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlagent-linux-arm:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vlagent-linux-arm64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vlagent-linux-ppc64le:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vlagent-linux-s390x:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vlagent-linux-loong64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vlagent-linux-386:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vlagent-darwin-amd64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlagent-darwin-arm64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vlagent-freebsd-amd64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlagent-openbsd-amd64:
	APP_NAME=vlagent CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vlagent-windows-amd64:
	GOARCH=amd64 APP_NAME=vlagent $(MAKE) app-local-windows-goarch

vlagent-pure:
	APP_NAME=vlagent $(MAKE) app-local-pure
