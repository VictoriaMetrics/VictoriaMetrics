# All these commands must run from repository root.

vmrestore:
	APP_NAME=vmrestore $(MAKE) app-local

vmrestore-race:
	APP_NAME=vmrestore RACE=-race $(MAKE) app-local

vmrestore-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker

vmrestore-pure-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-pure

vmrestore-amd64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-amd64

vmrestore-arm-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm

vmrestore-arm64-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-arm64

vmrestore-ppc64le-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-ppc64le

vmrestore-386-prod:
	APP_NAME=vmrestore $(MAKE) app-via-docker-386

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
