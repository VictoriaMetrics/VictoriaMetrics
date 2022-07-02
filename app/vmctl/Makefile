# All these commands must run from repository root.

# Application build
vmctl:
	APP_NAME=vmctl $(MAKE) app-local

vmctl-race:
	APP_NAME=vmctl RACE=-race $(MAKE) app-local

vmctl-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker

vmctl-pure-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-pure

# GOARCH=amd64
vmctl-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-amd64

vmctl-linux-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-amd64

vmctl-darwin-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-darwin-amd64

vmctl-windows-amd64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-windows-amd64

# GOARCH=arm64
vmctl-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-arm64

vmctl-linux-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-arm64

vmctl-darwin-arm64-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-darwin-arm64

# GOARCH=arm
vmctl-arm-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-arm

vmctl-linux-arm-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-linux-arm

# GOARCH=ppc64le
vmctl-ppc64le-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-ppc64le

# GOARCH=386
vmctl-386-prod:
	APP_NAME=vmctl $(MAKE) app-via-docker-386

# Package build
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

