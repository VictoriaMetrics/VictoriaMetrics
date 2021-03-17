# All these commands must run from repository root.

vmctl:
	APP_NAME=vmctl $(MAKE) app-local

vmctl-race:
	APP_NAME=vmctl RACE=-race $(MAKE) app-local

vmctl-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker

vmctl-pure-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-pure

vmctl-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-amd64

vmctl-arm-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-arm

vmctl-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-arm64

vmctl-ppc64le-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-ppc64le

vmctl-386-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-386

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

vmctl-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmctl-local-with-goarch

vmctl-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmctl-local-with-goarch

vmctl-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmctl-local-with-goarch

vmctl-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmctl-local-with-goarch

vmctl-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmctl-local-with-goarch

vmctl-local-with-goarch:
	APP_NAME=vmctl $(MAKE) app-local-with-goarch

vmctl-pure:
	APP_NAME=vmctl $(MAKE) app-local-pure

vmctl-windows-amd64:
	GOARCH=amd64 APP_NAME=vmctl $(MAKE) app-local-windows-with-goarch

vmctl-windows-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-windows-amd64
