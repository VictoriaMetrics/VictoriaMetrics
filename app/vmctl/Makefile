# All these commands must run from repository root.

vmctl:
	APP_NAME=vmctl $(MAKE) app-local

vmctl-race:
	APP_NAME=vmctl RACE=-race $(MAKE) app-local

vmctl-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker

vmctl-pure-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-pure

vmctl-linux-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-amd64

vmctl-linux-arm-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-arm

vmctl-linux-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-arm64

vmctl-linux-ppc64le-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-ppc64le

vmctl-linux-386-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-386

vmctl-darwin-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-darwin-amd64

vmctl-darwin-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-darwin-arm64

vmctl-freebsd-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-freebsd-amd64

vmctl-openbsd-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-openbsd-amd64

vmctl-windows-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-windows-amd64

package-vmctl:
	APP_NAME=vmctl $(MAKE) package-via-docker

package-vmctl-pure:
	APP_NAME=vmctl $(MAKE) package-via-docker-pure

package-vmctl-amd64:
	APP_NAME=vmctl $(MAKE) package-via-docker-amd64

package-vmctl-arm:
	APP_NAME=vmctl $(MAKE) package-via-docker-arm

package-vmctl-arm64:
	APP_NAME=vmctl $(MAKE) package-via-docker-arm64

package-vmctl-ppc64le:
	APP_NAME=vmctl $(MAKE) package-via-docker-ppc64le

package-vmctl-386:
	APP_NAME=vmctl $(MAKE) package-via-docker-386

publish-vmctl:
	APP_NAME=vmctl $(MAKE) publish-via-docker

vmctl-linux-amd64:
	APP_NAME=vmctl CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmctl-linux-arm:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmctl-linux-arm64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmctl-linux-ppc64le:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmctl-linux-s390x:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmctl-linux-loong64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmctl-linux-386:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmctl-darwin-amd64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmctl-darwin-arm64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmctl-freebsd-amd64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmctl-openbsd-amd64:
	APP_NAME=vmctl CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmctl-windows-amd64:
	GOARCH=amd64 APP_NAME=vmctl $(MAKE) app-local-windows-goarch

vmctl-pure:
	APP_NAME=vmctl $(MAKE) app-local-pure
