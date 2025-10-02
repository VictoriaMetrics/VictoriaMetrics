PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics

MAKE_CONCURRENCY ?= $(shell getconf _NPROCESSORS_ONLN)
MAKE_PARALLEL := $(MAKE) -j $(MAKE_CONCURRENCY)
DATEINFO_TAG ?= $(shell date -u +'%Y%m%d-%H%M%S')
BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -d' ' -f2 | cut -c 1-8)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

EXTRA_DOCKER_TAG_SUFFIX ?=
EXTRA_GO_BUILD_TAGS ?=

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(DATEINFO_TAG)-$(BUILDINFO_TAG)'
TAR_OWNERSHIP ?= --owner=1000 --group=1000

GOLANGCI_LINT_VERSION := 2.4.0

.PHONY: $(MAKECMDGOALS)

include app/*/Makefile
include codespell/Makefile
include docs/Makefile
include deployment/*/Makefile
include dashboards/Makefile
include package/release/Makefile
include benchmarks/Makefile

all: \
	victoria-metrics-prod \
	vmagent-prod \
	vmalert-prod \
	vmalert-tool-prod \
	vmauth-prod \
	vmbackup-prod \
	vmrestore-prod \
	vmctl-prod

clean:
	rm -rf bin/*

publish: \
	publish-victoria-metrics \
	publish-vmagent \
	publish-vmalert \
	publish-vmalert-tool \
	publish-vmauth \
	publish-vmbackup \
	publish-vmrestore \
	publish-vmctl

package: \
	package-victoria-metrics \
	package-vmagent \
	package-vmalert \
	package-vmalert-tool \
	package-vmauth \
	package-vmbackup \
	package-vmrestore \
	package-vmctl

vmutils: \
	vmagent \
	vmalert \
	vmalert-tool \
	vmauth \
	vmbackup \
	vmrestore \
	vmctl

vmutils-pure: \
	vmagent-pure \
	vmalert-pure \
	vmalert-tool-pure \
	vmauth-pure \
	vmbackup-pure \
	vmrestore-pure \
	vmctl-pure

vmutils-linux-amd64: \
	vmagent-linux-amd64 \
	vmalert-linux-amd64 \
	vmalert-tool-linux-amd64 \
	vmauth-linux-amd64 \
	vmbackup-linux-amd64 \
	vmrestore-linux-amd64 \
	vmctl-linux-amd64

vmutils-linux-arm64: \
	vmagent-linux-arm64 \
	vmalert-linux-arm64 \
	vmalert-tool-linux-arm64 \
	vmauth-linux-arm64 \
	vmbackup-linux-arm64 \
	vmrestore-linux-arm64 \
	vmctl-linux-arm64

vmutils-linux-arm: \
	vmagent-linux-arm \
	vmalert-linux-arm \
	vmalert-tool-linux-arm \
	vmauth-linux-arm \
	vmbackup-linux-arm \
	vmrestore-linux-arm \
	vmctl-linux-arm

vmutils-linux-386: \
	vmagent-linux-386 \
	vmalert-linux-386 \
	vmalert-tool-linux-386 \
	vmauth-linux-386 \
	vmbackup-linux-386 \
	vmrestore-linux-386 \
	vmctl-linux-386

vmutils-linux-ppc64le: \
	vmagent-linux-ppc64le \
	vmalert-linux-ppc64le \
	vmalert-tool-linux-ppc64le \
	vmauth-linux-ppc64le \
	vmbackup-linux-ppc64le \
	vmrestore-linux-ppc64le \
	vmctl-linux-ppc64le

vmutils-darwin-amd64: \
	vmagent-darwin-amd64 \
	vmalert-darwin-amd64 \
	vmalert-tool-darwin-amd64 \
	vmauth-darwin-amd64 \
	vmbackup-darwin-amd64 \
	vmrestore-darwin-amd64 \
	vmctl-darwin-amd64

vmutils-darwin-arm64: \
	vmagent-darwin-arm64 \
	vmalert-darwin-arm64 \
	vmalert-tool-darwin-arm64 \
	vmauth-darwin-arm64 \
	vmbackup-darwin-arm64 \
	vmrestore-darwin-arm64 \
	vmctl-darwin-arm64

vmutils-freebsd-amd64: \
	vmagent-freebsd-amd64 \
	vmalert-freebsd-amd64 \
	vmalert-tool-freebsd-amd64 \
	vmauth-freebsd-amd64 \
	vmbackup-freebsd-amd64 \
	vmrestore-freebsd-amd64 \
	vmctl-freebsd-amd64

vmutils-openbsd-amd64: \
	vmagent-openbsd-amd64 \
	vmalert-openbsd-amd64 \
	vmalert-tool-openbsd-amd64 \
	vmauth-openbsd-amd64 \
	vmbackup-openbsd-amd64 \
	vmrestore-openbsd-amd64 \
	vmctl-openbsd-amd64

vmutils-windows-amd64: \
	vmagent-windows-amd64 \
	vmalert-windows-amd64 \
	vmalert-tool-windows-amd64 \
	vmauth-windows-amd64 \
	vmbackup-windows-amd64 \
	vmrestore-windows-amd64 \
	vmctl-windows-amd64

# When adding a new crossbuild target, please also add it to the .github/workflows/build.yml
crossbuild:
	$(MAKE_PARALLEL) victoria-metrics-crossbuild vmutils-crossbuild

# When adding a new crossbuild target, please also add it to the .github/workflows/build.yml
victoria-metrics-crossbuild: \
	victoria-metrics-linux-386 \
	victoria-metrics-linux-amd64 \
	victoria-metrics-linux-arm64 \
	victoria-metrics-linux-arm \
	victoria-metrics-linux-ppc64le \
	victoria-metrics-darwin-amd64 \
	victoria-metrics-darwin-arm64 \
	victoria-metrics-freebsd-amd64 \
	victoria-metrics-openbsd-amd64 \
	victoria-metrics-windows-amd64

# When adding a new crossbuild target, please also add it to the .github/workflows/build.yml
vmutils-crossbuild: \
	vmutils-linux-386 \
	vmutils-linux-amd64 \
	vmutils-linux-arm64 \
	vmutils-linux-arm \
	vmutils-linux-ppc64le \
	vmutils-darwin-amd64 \
	vmutils-darwin-arm64 \
	vmutils-freebsd-amd64 \
	vmutils-openbsd-amd64 \
	vmutils-windows-amd64

publish-final-images:
	PKG_TAG=$(TAG) APP_NAME=victoria-metrics $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmagent $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmalert $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmalert-tool $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmauth $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmbackup $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmrestore $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) APP_NAME=vmctl $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vminsert $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vmselect $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vmstorage $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=victoria-metrics $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmagent $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmalert $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmauth $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmbackup $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmrestore $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise-cluster APP_NAME=vminsert $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise-cluster APP_NAME=vmselect $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise-cluster APP_NAME=vmstorage $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmgateway $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmbackupmanager $(MAKE) publish-via-docker-from-rc && \
	PKG_TAG=$(TAG) $(MAKE) publish-latest

publish-latest:
	PKG_TAG=$(TAG) APP_NAME=victoria-metrics $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmagent $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmalert $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmalert-tool $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmauth $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmbackup $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmrestore $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG) APP_NAME=vmctl $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vminsert $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vmselect $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG)-cluster APP_NAME=vmstorage $(MAKE) publish-via-docker-latest && \
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmgateway $(MAKE) publish-via-docker-latest
	PKG_TAG=$(TAG)-enterprise APP_NAME=vmbackupmanager $(MAKE) publish-via-docker-latest

publish-release:
	rm -rf bin/*
	git checkout $(TAG) && $(MAKE) release && $(MAKE) publish && \
		git checkout $(TAG)-cluster && $(MAKE) release && $(MAKE) publish && \
		git checkout $(TAG)-enterprise && $(MAKE) release && $(MAKE) publish && \
		git checkout $(TAG)-enterprise-cluster && $(MAKE) release && $(MAKE) publish

release:
	$(MAKE_PARALLEL) \
		release-victoria-metrics \
		release-vmutils

release-victoria-metrics: \
	release-victoria-metrics-linux-386 \
	release-victoria-metrics-linux-amd64 \
	release-victoria-metrics-linux-arm \
	release-victoria-metrics-linux-arm64 \
	release-victoria-metrics-darwin-amd64 \
	release-victoria-metrics-darwin-arm64 \
	release-victoria-metrics-freebsd-amd64 \
	release-victoria-metrics-openbsd-amd64 \
	release-victoria-metrics-windows-amd64

release-victoria-metrics-linux-386:
	GOOS=linux GOARCH=386 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-linux-arm:
	GOOS=linux GOARCH=arm $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-linux-arm64:
	GOOS=linux GOARCH=arm64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-openbsd-amd64:
	GOOS=openbsd GOARCH=amd64 $(MAKE) release-victoria-metrics-goos-goarch

release-victoria-metrics-windows-amd64:
	GOARCH=amd64 $(MAKE) release-victoria-metrics-windows-goarch

release-victoria-metrics-goos-goarch: victoria-metrics-$(GOOS)-$(GOARCH)-prod
	cd bin && \
		tar $(TAR_OWNERSHIP) --transform="flags=r;s|-$(GOOS)-$(GOARCH)||" -czf victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(GOOS)-$(GOARCH)-prod \
		&& sha256sum victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(GOOS)-$(GOARCH)-prod \
			| sed s/-$(GOOS)-$(GOARCH)-prod/-prod/ > victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf victoria-metrics-$(GOOS)-$(GOARCH)-prod

release-victoria-metrics-windows-goarch: victoria-metrics-windows-$(GOARCH)-prod
	cd bin && \
		zip victoria-metrics-windows-$(GOARCH)-$(PKG_TAG).zip \
			victoria-metrics-windows-$(GOARCH)-prod.exe \
		&& sha256sum victoria-metrics-windows-$(GOARCH)-$(PKG_TAG).zip \
			victoria-metrics-windows-$(GOARCH)-prod.exe \
			> victoria-metrics-windows-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		victoria-metrics-windows-$(GOARCH)-prod.exe

release-vmutils: \
	release-vmutils-linux-386 \
	release-vmutils-linux-amd64 \
	release-vmutils-linux-arm64 \
	release-vmutils-linux-arm \
	release-vmutils-darwin-amd64 \
	release-vmutils-darwin-arm64 \
	release-vmutils-freebsd-amd64 \
	release-vmutils-openbsd-amd64 \
	release-vmutils-windows-amd64

release-vmutils-linux-386:
	GOOS=linux GOARCH=386 $(MAKE) release-vmutils-goos-goarch

release-vmutils-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-linux-arm64:
	GOOS=linux GOARCH=arm64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-linux-arm:
	GOOS=linux GOARCH=arm $(MAKE) release-vmutils-goos-goarch

release-vmutils-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-openbsd-amd64:
	GOOS=openbsd GOARCH=amd64 $(MAKE) release-vmutils-goos-goarch

release-vmutils-windows-amd64:
	GOARCH=amd64 $(MAKE) release-vmutils-windows-goarch

release-vmutils-goos-goarch: \
	vmagent-$(GOOS)-$(GOARCH)-prod \
	vmalert-$(GOOS)-$(GOARCH)-prod \
	vmalert-tool-$(GOOS)-$(GOARCH)-prod \
	vmauth-$(GOOS)-$(GOARCH)-prod \
	vmbackup-$(GOOS)-$(GOARCH)-prod \
	vmrestore-$(GOOS)-$(GOARCH)-prod \
	vmctl-$(GOOS)-$(GOARCH)-prod
	cd bin && \
		tar $(TAR_OWNERSHIP) --transform="flags=r;s|-$(GOOS)-$(GOARCH)||" -czf vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(GOOS)-$(GOARCH)-prod \
			vmalert-$(GOOS)-$(GOARCH)-prod \
			vmalert-tool-$(GOOS)-$(GOARCH)-prod \
			vmauth-$(GOOS)-$(GOARCH)-prod \
			vmbackup-$(GOOS)-$(GOARCH)-prod \
			vmrestore-$(GOOS)-$(GOARCH)-prod \
			vmctl-$(GOOS)-$(GOARCH)-prod \
		&& sha256sum vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(GOOS)-$(GOARCH)-prod \
			vmalert-$(GOOS)-$(GOARCH)-prod \
			vmalert-tool-$(GOOS)-$(GOARCH)-prod \
			vmauth-$(GOOS)-$(GOARCH)-prod \
			vmbackup-$(GOOS)-$(GOARCH)-prod \
			vmrestore-$(GOOS)-$(GOARCH)-prod \
			vmctl-$(GOOS)-$(GOARCH)-prod \
			| sed s/-$(GOOS)-$(GOARCH)-prod/-prod/ > vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		vmagent-$(GOOS)-$(GOARCH)-prod \
		vmalert-$(GOOS)-$(GOARCH)-prod \
		vmalert-tool-$(GOOS)-$(GOARCH)-prod \
		vmauth-$(GOOS)-$(GOARCH)-prod \
		vmbackup-$(GOOS)-$(GOARCH)-prod \
		vmrestore-$(GOOS)-$(GOARCH)-prod \
		vmctl-$(GOOS)-$(GOARCH)-prod

release-vmutils-windows-goarch: \
	vmagent-windows-$(GOARCH)-prod \
	vmalert-windows-$(GOARCH)-prod \
	vmalert-tool-windows-$(GOARCH)-prod \
	vmauth-windows-$(GOARCH)-prod \
	vmbackup-windows-$(GOARCH)-prod \
	vmrestore-windows-$(GOARCH)-prod \
	vmctl-windows-$(GOARCH)-prod
	cd bin && \
		zip vmutils-windows-$(GOARCH)-$(PKG_TAG).zip \
			vmagent-windows-$(GOARCH)-prod.exe \
			vmalert-windows-$(GOARCH)-prod.exe \
			vmalert-tool-windows-$(GOARCH)-prod.exe \
			vmauth-windows-$(GOARCH)-prod.exe \
			vmbackup-windows-$(GOARCH)-prod.exe \
			vmrestore-windows-$(GOARCH)-prod.exe \
			vmctl-windows-$(GOARCH)-prod.exe \
		&& sha256sum vmutils-windows-$(GOARCH)-$(PKG_TAG).zip \
			vmagent-windows-$(GOARCH)-prod.exe \
			vmalert-windows-$(GOARCH)-prod.exe \
			vmalert-tool-windows-$(GOARCH)-prod.exe \
			vmauth-windows-$(GOARCH)-prod.exe \
			vmbackup-windows-$(GOARCH)-prod.exe \
			vmrestore-windows-$(GOARCH)-prod.exe \
			vmctl-windows-$(GOARCH)-prod.exe \
			> vmutils-windows-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		vmagent-windows-$(GOARCH)-prod.exe \
		vmalert-windows-$(GOARCH)-prod.exe \
		vmalert-tool-windows-$(GOARCH)-prod.exe \
		vmauth-windows-$(GOARCH)-prod.exe \
		vmbackup-windows-$(GOARCH)-prod.exe \
		vmrestore-windows-$(GOARCH)-prod.exe \
		vmctl-windows-$(GOARCH)-prod.exe

pprof-cpu:
	go tool pprof -trim_path=github.com/VictoriaMetrics/VictoriaMetrics@ $(PPROF_FILE)

fmt:
	gofmt -l -w -s ./lib
	gofmt -l -w -s ./app
	gofmt -l -w -s ./apptest

vet:
	GOEXPERIMENT=synctest go vet ./lib/...
	go vet ./app/...
	go vet ./apptest/...

check-all: fmt vet golangci-lint govulncheck

clean-checkers: remove-golangci-lint remove-govulncheck

test:
	GOEXPERIMENT=synctest go test ./lib/... ./app/...

test-race:
	GOEXPERIMENT=synctest go test -race ./lib/... ./app/...

test-pure:
	GOEXPERIMENT=synctest CGO_ENABLED=0 go test ./lib/... ./app/...

test-full:
	GOEXPERIMENT=synctest go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

test-full-386:
	GOEXPERIMENT=synctest GOARCH=386 go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

integration-test:
	$(MAKE) apptest

apptest:
	$(MAKE) victoria-metrics vmagent vmalert vmauth vmctl vmbackup vmrestore
	go test ./apptest/... -skip="^TestCluster.*"

benchmark:
	GOEXPERIMENT=synctest go test -bench=. ./lib/...
	go test -bench=. ./app/...

benchmark-pure:
	GOEXPERIMENT=synctest CGO_ENABLED=0 go test -bench=. ./lib/...
	CGO_ENABLED=0 go test -bench=. ./app/...

vendor-update:
	go get -u ./lib/...
	go get -u ./app/...
	go mod tidy -compat=1.24
	go mod vendor

app-local:
	CGO_ENABLED=1 go build $(RACE) -ldflags "$(GO_BUILDINFO)" -tags "$(EXTRA_GO_BUILD_TAGS)" -o bin/$(APP_NAME)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-pure:
	CGO_ENABLED=0 go build $(RACE) -ldflags "$(GO_BUILDINFO)" -tags "$(EXTRA_GO_BUILD_TAGS)" -o bin/$(APP_NAME)-pure$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-goos-goarch:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(RACE) -ldflags "$(GO_BUILDINFO)" -tags "$(EXTRA_GO_BUILD_TAGS)" -o bin/$(APP_NAME)-$(GOOS)-$(GOARCH)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-windows-goarch:
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build $(RACE) -ldflags "$(GO_BUILDINFO)" -tags "$(EXTRA_GO_BUILD_TAGS)" -o bin/$(APP_NAME)-windows-$(GOARCH)$(RACE).exe $(PKG_PREFIX)/app/$(APP_NAME)

quicktemplate-gen: install-qtc
	qtc

install-qtc:
	which qtc || go install github.com/valyala/quicktemplate/qtc@latest


golangci-lint: install-golangci-lint
	GOEXPERIMENT=synctest golangci-lint run

install-golangci-lint:
	which golangci-lint && (golangci-lint --version | grep -q $(GOLANGCI_LINT_VERSION)) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v$(GOLANGCI_LINT_VERSION)

remove-golangci-lint:
	rm -rf `which golangci-lint`

govulncheck: install-govulncheck
	govulncheck ./...

install-govulncheck:
	which govulncheck || go install golang.org/x/vuln/cmd/govulncheck@latest

remove-govulncheck:
	rm -rf `which govulncheck`

install-wwhrd:
	which wwhrd || go install github.com/frapposelli/wwhrd@latest

check-licenses: install-wwhrd
	wwhrd check -f .wwhrd.yml
