# All these commands must run from repository root.

# Application build
vmrestore:
	APP_NAME=vmrestore $(MAKE) app-local

vmrestore-race:
	APP_NAME=vmrestore RACE=-race $(MAKE) app-local

vmrestore-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker

vmrestore-pure-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-pure

# GOARCH=amd64
vmrestore-amd64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-amd64

vmrestore-linux-amd64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-linux-amd64

vmrestore-darwin-amd64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-darwin-amd64

# GOARCH=arm64
vmrestore-arm64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm64

vmrestore-linux-arm64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-linux-arm64

vmrestore-darwin-arm64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-darwin-arm64

# GOARCH=arm
vmrestore-arm-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm

vmrestore-linux-arm-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-linux-arm

# GOARCH=ppc64le
vmrestore-ppc64le-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-ppc64le

# GOARCH=386
vmrestore-386-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-386

# Package build
package-vmrestore:
	APP_NAME=vmrestore $(MAKE) package-via-docker

package-vmrestore-pure:
	APP_NAME=vmrestore $(MAKE) package-via-docker-pure

package-vmrestore-amd64:
	APP_NAME=vmrestore $(MAKE) package-via-docker-amd64

package-vmrestore-arm:
	APP_NAME=vmrestore $(MAKE) package-via-docker-arm

package-vmrestore-arm64:
	APP_NAME=vmrestore $(MAKE) package-via-docker-arm64

package-vmrestore-ppc64le:
	APP_NAME=vmrestore $(MAKE) package-via-docker-ppc64le

package-vmrestore-386:
	APP_NAME=vmrestore $(MAKE) package-via-docker-386

publish-vmrestore:
	APP_NAME=vmrestore $(MAKE) publish-via-docker

vmrestore-amd64:
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) vmrestore-local-with-goarch

vmrestore-arm:
	CGO_ENABLED=0 GOARCH=arm $(MAKE) vmrestore-local-with-goarch

vmrestore-arm64:
	CGO_ENABLED=0 GOARCH=arm64 $(MAKE) vmrestore-local-with-goarch

vmrestore-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) vmrestore-local-with-goarch

vmrestore-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) vmrestore-local-with-goarch

vmrestore-local-with-goarch:
	APP_NAME=vmrestore $(MAKE) app-local-with-goarch

vmrestore-pure:
	APP_NAME=vmrestore $(MAKE) app-local-pure
