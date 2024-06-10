# All these commands must run from repository root.

vmbackup:
	APP_NAME=vmbackup $(MAKE) app-local

vmbackup-race:
	APP_NAME=vmbackup RACE=-race $(MAKE) app-local

vmbackup-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker

vmbackup-pure-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-pure

vmbackup-linux-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-linux-amd64

vmbackup-linux-arm-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-linux-arm

vmbackup-linux-arm64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-linux-arm64

vmbackup-linux-ppc64le-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-linux-ppc64le

vmbackup-linux-386-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-linux-386

vmbackup-darwin-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-darwin-amd64

vmbackup-darwin-arm64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-darwin-arm64

vmbackup-freebsd-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-freebsd-amd64

vmbackup-openbsd-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-openbsd-amd64

vmbackup-windows-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-windows-amd64

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

vmbackup-linux-amd64:
	APP_NAME=vmbackup CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmbackup-linux-arm:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-local-goos-goarch

vmbackup-linux-arm64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmbackup-linux-ppc64le:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-local-goos-goarch

vmbackup-linux-s390x:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-local-goos-goarch

vmbackup-linux-loong64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=loong64 $(MAKE) app-local-goos-goarch

vmbackup-linux-386:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-local-goos-goarch

vmbackup-darwin-amd64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmbackup-darwin-arm64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-local-goos-goarch

vmbackup-freebsd-amd64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmbackup-openbsd-amd64:
	APP_NAME=vmbackup CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-local-goos-goarch

vmbackup-windows-amd64:
	GOARCH=amd64 APP_NAME=vmbackup $(MAKE) app-local-windows-goarch

vmbackup-pure:
	APP_NAME=vmbackup $(MAKE) app-local-pure
