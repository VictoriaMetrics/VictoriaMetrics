# All these commands must run from repository root.

vmbackup:
	APP_NAME=vmbackup $(MAKE) app-local

vmbackup-race:
	APP_NAME=vmbackup RACE=-race $(MAKE) app-local

vmbackup-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker

vmbackup-pure-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-pure

vmbackup-amd64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-amd64

vmbackup-arm-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-arm

vmbackup-arm64-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-arm64

vmbackup-ppc64le-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-ppc64le

vmbackup-386-prod:
	APP_NAME=vmbackup $(MAKE) app-via-docker-386

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

vmbackup-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmbackup-local-with-goarch

vmbackup-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmbackup-local-with-goarch

vmbackup-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmbackup-local-with-goarch

vmbackup-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmbackup-local-with-goarch

vmbackup-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmbackup-local-with-goarch

vmbackup-local-with-goarch:
	APP_NAME=vmbackup $(MAKE) app-local-with-goarch

vmbackup-pure:
	APP_NAME=vmbackup $(MAKE) app-local-pure
