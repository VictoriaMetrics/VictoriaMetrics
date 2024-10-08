PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics

MAKE_CONCURRENCY ?= $(shell getconf _NPROCESSORS_ONLN)
MAKE_PARALLEL := $(MAKE) -j $(MAKE_CONCURRENCY)
DATEINFO_TAG ?= $(shell date -u +'%Y%m%d-%H%M%S')
BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -d' ' -f2 | cut -c 1-8)))
LATEST_TAG ?= cluster-latest

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(DATEINFO_TAG)-$(BUILDINFO_TAG)'
TAR_OWNERSHIP ?= --owner=1000 --group=1000

.PHONY: $(MAKECMDGOALS)

include app/*/Makefile
include cspell/Makefile
include docs/Makefile
include deployment/*/Makefile
include dashboards/Makefile
include package/release/Makefile

all: \
	vminsert \
	vmselect \
	vmstorage

all-pure: \
	vminsert-pure \
	vmselect-pure \
	vmstorage-pure

clean:
	rm -rf bin/*

vmcluster-linux-amd64: \
	vminsert-linux-amd64 \
	vmselect-linux-amd64 \
	vmstorage-linux-amd64

vmcluster-linux-arm64: \
	vminsert-linux-arm64 \
	vmselect-linux-arm64 \
	vmstorage-linux-arm64

vmcluster-linux-arm: \
	vminsert-linux-arm \
	vmselect-linux-arm \
	vmstorage-linux-arm

vmcluster-linux-ppc64le: \
	vminsert-linux-ppc64le \
	vmselect-linux-ppc64le \
	vmstorage-linux-ppc64le

vmcluster-linux-386: \
	vminsert-linux-386 \
	vmselect-linux-386 \
	vmstorage-linux-386

vmcluster-freebsd-amd64: \
	vminsert-freebsd-amd64 \
	vmselect-freebsd-amd64 \
	vmstorage-freebsd-amd64

vmcluster-openbsd-amd64: \
	vminsert-openbsd-amd64 \
	vmselect-openbsd-amd64 \
	vmstorage-openbsd-amd64

vmcluster-windows-amd64: \
	vminsert-windows-amd64 \
	vmselect-windows-amd64 \
	vmstorage-windows-amd64

vmcluster-darwin-amd64: \
	vminsert-darwin-amd64 \
	vmselect-darwin-amd64 \
	vmstorage-darwin-amd64

vmcluster-darwin-arm64: \
	vminsert-darwin-arm64 \
	vmselect-darwin-arm64 \
	vmstorage-darwin-arm64

crossbuild: vmcluster-crossbuild

vmcluster-crossbuild:
	$(MAKE_PARALLEL) vmcluster-linux-amd64 \
		vmcluster-linux-arm64 \
		vmcluster-linux-arm \
		vmcluster-linux-ppc64le \
		vmcluster-linux-386 \
		vmcluster-freebsd-amd64 \
		vmcluster-openbsd-amd64

publish: \
	publish-vminsert \
	publish-vmselect \
	publish-vmstorage

package: \
	package-vminsert \
	package-vmselect \
	package-vmstorage

publish-release:
	rm -rf bin/*
	git checkout $(TAG) && $(MAKE) release && LATEST_TAG=stable $(MAKE) publish && \
		git checkout $(TAG)-cluster && $(MAKE) release && LATEST_TAG=cluster-stable $(MAKE) publish && \
		git checkout $(TAG)-enterprise && $(MAKE) release && LATEST_TAG=enterprise-stable $(MAKE) publish && \
		git checkout $(TAG)-enterprise-cluster && $(MAKE) release && LATEST_TAG=enterprise-cluster-stable $(MAKE) publish

release:
	$(MAKE_PARALLEL) release-vmcluster

release-vmcluster: \
	release-vmcluster-linux-amd64 \
	release-vmcluster-linux-arm64 \
	release-vmcluster-freebsd-amd64 \
	release-vmcluster-openbsd-amd64 \
	release-vmcluster-windows-amd64 \
	release-vmcluster-darwin-amd64 \
	release-vmcluster-darwin-arm64

release-vmcluster-linux-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-linux-arm64:
	GOOS=linux GOARCH=arm64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-freebsd-amd64:
	GOOS=freebsd GOARCH=amd64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-openbsd-amd64:
	GOOS=openbsd GOARCH=amd64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-windows-amd64:
	GOARCH=amd64 $(MAKE) release-vmcluster-windows-goarch

release-vmcluster-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-darwin-arm64:
	 GOOS=darwin GOARCH=arm64 $(MAKE) release-vmcluster-goos-goarch

release-vmcluster-goos-goarch: \
	vminsert-$(GOOS)-$(GOARCH)-prod \
	vmselect-$(GOOS)-$(GOARCH)-prod \
	vmstorage-$(GOOS)-$(GOARCH)-prod
	cd bin && \
		tar $(TAR_OWNERSHIP) --transform="flags=r;s|-$(GOOS)-$(GOARCH)||" -czf victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vminsert-$(GOOS)-$(GOARCH)-prod \
			vmselect-$(GOOS)-$(GOARCH)-prod \
			vmstorage-$(GOOS)-$(GOARCH)-prod \
		&& sha256sum victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vminsert-$(GOOS)-$(GOARCH)-prod \
			vmselect-$(GOOS)-$(GOARCH)-prod \
			vmstorage-$(GOOS)-$(GOARCH)-prod \
			| sed s/-$(GOOS)-$(GOARCH)-prod/-prod/ > victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		vminsert-$(GOOS)-$(GOARCH)-prod \
		vmselect-$(GOOS)-$(GOARCH)-prod \
		vmstorage-$(GOOS)-$(GOARCH)-prod

release-vmcluster-windows-goarch: \
	vminsert-windows-$(GOARCH)-prod \
	vmselect-windows-$(GOARCH)-prod \
	vmstorage-windows-$(GOARCH)-prod
	cd bin && \
		zip victoria-metrics-windows-$(GOARCH)-$(PKG_TAG).zip \
			vminsert-windows-$(GOARCH)-prod.exe \
			vmselect-windows-$(GOARCH)-prod.exe \
			vmstorage-windows-$(GOARCH)-prod.exe \
		&& sha256sum victoria-metrics-windows-$(GOARCH)-$(PKG_TAG).zip \
			vminsert-windows-$(GOARCH)-prod.exe \
			vmselect-windows-$(GOARCH)-prod.exe \
			vmstorage-windows-$(GOARCH)-prod.exe \
		> victoria-metrics-windows-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		vminsert-windows-$(GOARCH)-prod.exe \
		vmselect-windows-$(GOARCH)-prod.exe \
		vmstorage-windows-$(GOARCH)-prod.exe

pprof-cpu:
	go tool pprof -trim_path=github.com/VictoriaMetrics/VictoriaMetrics@ $(PPROF_FILE)

fmt:
	gofmt -l -w -s ./lib
	gofmt -l -w -s ./app

vet:
	go vet ./lib/...
	go vet ./app/...

check-all: fmt vet golangci-lint govulncheck

clean-checkers: remove-golangci-lint remove-govulncheck

test:
	DISABLE_FSYNC_FOR_TESTING=1 go test ./lib/... ./app/...

test-race:
	DISABLE_FSYNC_FOR_TESTING=1 go test -race ./lib/... ./app/...

test-pure:
	DISABLE_FSYNC_FOR_TESTING=1 CGO_ENABLED=0 go test ./lib/... ./app/...

test-full:
	DISABLE_FSYNC_FOR_TESTING=1 go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

test-full-386:
	DISABLE_FSYNC_FOR_TESTING=1 GOARCH=386 go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

benchmark:
	go test -bench=. ./lib/...
	go test -bench=. ./app/...

benchmark-pure:
	CGO_ENABLED=0 go test -bench=. ./lib/...
	CGO_ENABLED=0 go test -bench=. ./app/...

vendor-update:
	go get -u ./lib/...
	go get -u ./app/...
	go mod tidy -compat=1.23
	go mod vendor

app-local:
	CGO_ENABLED=1 go build $(RACE) -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-pure:
	CGO_ENABLED=0 go build $(RACE) -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-pure$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-goos-goarch:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(RACE) -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-$(GOOS)-$(GOARCH)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-windows-goarch:
	CGO_ENABLED=0 GOOS=windows GOARCH=$(GOARCH) go build $(RACE) -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-windows-$(GOARCH)$(RACE).exe $(PKG_PREFIX)/app/$(APP_NAME)

quicktemplate-gen: install-qtc
	qtc

install-qtc:
	which qtc || go install github.com/valyala/quicktemplate/qtc@latest


golangci-lint: install-golangci-lint
	golangci-lint run

install-golangci-lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.60.3

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
