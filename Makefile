PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics

DATEINFO_TAG ?= $(shell date -u +'%Y%m%d-%H%M%S')
BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -c 10-17)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(DATEINFO_TAG)-$(BUILDINFO_TAG)'

.PHONY: $(MAKECMDGOALS)

all: \
	victoria-metrics-prod \
	vmagent-prod \
	vmalert-prod \
	vmauth-prod \
	vmbackup-prod \
	vmrestore-prod \
	vmctl-prod

include app/*/Makefile
include deployment/*/Makefile
include snap/local/Makefile


clean:
	rm -rf bin/*

publish: \
	publish-victoria-metrics \
	publish-vmagent \
	publish-vmalert \
	publish-vmauth \
	publish-vmbackup \
	publish-vmrestore \
	publish-vmctl

package: \
	package-victoria-metrics \
	package-vmagent \
	package-vmalert \
	package-vmauth \
	package-vmbackup \
	package-vmrestore \
	package-vmctl

vmutils: \
	vmagent \
	vmalert \
	vmauth \
	vmbackup \
	vmrestore \
	vmctl

vmutils-pure: \
	vmagent-pure \
	vmalert-pure \
	vmauth-pure \
	vmbackup-pure \
	vmrestore-pure \
	vmctl-pure

vmutils-arm64: \
	vmagent-arm64 \
	vmalert-arm64 \
	vmauth-arm64 \
	vmbackup-arm64 \
	vmrestore-arm64 \
	vmctl-arm64

vmutils-arm: \
	vmagent-arm \
	vmalert-arm \
	vmauth-arm \
	vmbackup-arm \
	vmrestore-arm \
	vmctl-arm

vmutils-windows-amd64: \
	vmagent-windows-amd64 \
	vmalert-windows-amd64 \
	vmauth-windows-amd64 \
	vmctl-windows-amd64


publish-release:
	git checkout $(TAG) && $(MAKE) release copy-linux publish && \
	git checkout $(TAG)-cluster && $(MAKE) release copy-linux-cluster publish && \
	git checkout $(TAG)-enterprise && $(MAKE) release copy-linux-enterprise publish && \
	git checkout $(TAG)-enterprise-cluster && $(MAKE) release copy-linux-enterprise-cluster publish

release: \
	release-victoria-metrics \
	release-vmutils

release-victoria-metrics: \
	release-victoria-metrics-linux-amd64 \
	release-victoria-metrics-linux-arm \
	release-victoria-metrics-linux-arm64 \
	release-victoria-metrics-darwin-amd64 \
	release-victoria-metrics-darwin-arm64

release-victoria-metrics-linux-amd64:
	OSARCH=linux-amd64 $(MAKE) release-victoria-metrics-generic

release-victoria-metrics-darwin-amd64:
	OSARCH=darwin-amd64 $(MAKE) release-victoria-metrics-generic

release-victoria-metrics-linux-arm:
	OSARCH=linux-arm $(MAKE) release-victoria-metrics-generic

release-victoria-metrics-linux-arm64:
	OSARCH=linux-arm64 $(MAKE) release-victoria-metrics-generic

release-victoria-metrics-darwin-arm64:
	OSARCH=darwin-arm64 $(MAKE) release-victoria-metrics-generic

release-victoria-metrics-generic: victoria-metrics-$(OSARCH)-prod
	cd bin && \
		gtar --transform="flags=r;s|-$(OSARCH)||" -czf victoria-metrics-$(OSARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(OSARCH)-prod \
		&& sha256sum victoria-metrics-$(OSARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(OSARCH)-prod \
			| sed s/-$(OSARCH)-prod/-prod/ > victoria-metrics-$(OSARCH)-$(PKG_TAG)_checksums.txt

release-vmutils: \
	release-vmutils-linux-amd64 \
	release-vmutils-linux-arm \
	release-vmutils-linux-arm64 \
	release-vmutils-darwin-amd64 \
	release-vmutils-darwin-arm64 \
	release-vmutils-windows-amd64

release-vmutils-linux-amd64:
	OSARCH=linux-amd64 $(MAKE) release-vmutils-generic

release-vmutils-darwin-amd64:
	OSARCH=darwin-amd64 $(MAKE) release-vmutils-generic

release-vmutils-windows-amd64:
	OSARCH=windows-amd64 $(MAKE) release-vmutils-windows-generic

release-vmutils-linux-arm:
	OSARCH=linux-arm $(MAKE) release-vmutils-generic

release-vmutils-linux-arm64:
	OSARCH=linux-arm64 $(MAKE) release-vmutils-generic

release-vmutils-darwin-arm64:
	OSARCH=darwin-arm64 $(MAKE) release-vmutils-generic

release-vmutils-generic: \
	vmagent-$(OSARCH)-prod \
	vmalert-$(OSARCH)-prod \
	vmauth-$(OSARCH)-prod \
	vmbackup-$(OSARCH)-prod \
	vmrestore-$(OSARCH)-prod \
	vmctl-$(OSARCH)-prod
	cd bin && \
		gtar --transform="flags=r;s|-$(OSARCH)||" -czf vmutils-$(OSARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(OSARCH)-prod \
			vmalert-$(OSARCH)-prod \
			vmauth-$(OSARCH)-prod \
			vmbackup-$(OSARCH)-prod \
			vmrestore-$(OSARCH)-prod \
			vmctl-$(OSARCH)-prod \
		&& sha256sum vmutils-$(OSARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(OSARCH)-prod \
			vmalert-$(OSARCH)-prod \
			vmauth-$(OSARCH)-prod \
			vmbackup-$(OSARCH)-prod \
			vmrestore-$(OSARCH)-prod \
			vmctl-$(OSARCH)-prod \
			| sed s/-$(OSARCH)-prod/-prod/ > vmutils-$(OSARCH)-$(PKG_TAG)_checksums.txt

release-vmutils-windows-generic: \
	vmagent-windows-$(GOARCH)-prod \
	vmalert-windows-$(GOARCH)-prod \
	vmauth-windows-$(GOARCH)-prod \
	vmctl-windows-$(GOARCH)-prod
	cd bin && \
		zip vmutils-windows-$(GOARCH)-$(PKG_TAG).zip \
			vmagent-windows-$(GOARCH)-prod.exe \
			vmalert-windows-$(GOARCH)-prod.exe \
			vmauth-windows-$(GOARCH)-prod.exe \
			vmctl-windows-$(GOARCH)-prod.exe \
		&& sha256sum vmutils-windows-$(GOARCH)-$(PKG_TAG).zip \
			vmagent-windows-$(GOARCH)-prod.exe \
			vmalert-windows-$(GOARCH)-prod.exe \
			vmauth-windows-$(GOARCH)-prod.exe \
			vmctl-windows-$(GOARCH)-prod.exe \
			> vmutils-windows-$(GOARCH)-$(PKG_TAG)_checksums.txt


pprof-cpu:
	go tool pprof -trim_path=github.com/VictoriaMetrics/VictoriaMetrics@ $(PPROF_FILE)

fmt:
	GO111MODULE=on gofmt -l -w -s ./lib
	GO111MODULE=on gofmt -l -w -s ./app

vet:
	GO111MODULE=on go vet -mod=vendor ./lib/...
	GO111MODULE=on go vet -mod=vendor ./app/...

lint: install-golint
	golint lib/...
	golint app/...

install-golint:
	which golint || GO111MODULE=off go get golang.org/x/lint/golint

errcheck: install-errcheck
	errcheck -exclude=errcheck_excludes.txt ./lib/...
	errcheck -exclude=errcheck_excludes.txt ./app/vminsert/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmselect/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmstorage/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmagent/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmalert/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmauth/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmbackup/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmrestore/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmctl/...

install-errcheck:
	which errcheck || GO111MODULE=off go get github.com/kisielk/errcheck

check-all: fmt vet lint errcheck golangci-lint

test:
	GO111MODULE=on go test -mod=vendor ./lib/... ./app/...

test-race:
	GO111MODULE=on go test -mod=vendor -race ./lib/... ./app/...

test-pure:
	GO111MODULE=on CGO_ENABLED=0 go test -mod=vendor ./lib/... ./app/...

test-full:
	GO111MODULE=on go test -mod=vendor -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

test-full-386:
	GO111MODULE=on GOARCH=386 go test -mod=vendor -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

benchmark:
	GO111MODULE=on go test -mod=vendor -bench=. ./lib/...
	GO111MODULE=on go test -mod=vendor -bench=. ./app/...

benchmark-pure:
	GO111MODULE=on CGO_ENABLED=0 go test -mod=vendor -bench=. ./lib/...
	GO111MODULE=on CGO_ENABLED=0 go test -mod=vendor -bench=. ./app/...

vendor-update:
	GO111MODULE=on go get -u -d ./lib/...
	GO111MODULE=on go get -u -d ./app/...
	GO111MODULE=on go mod tidy -compat=1.17
	GO111MODULE=on go mod vendor

app-local:
	CGO_ENABLED=1 GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-pure:
	CGO_ENABLED=0 GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-pure$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-with-goarch:
	GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-$(GOARCH)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

app-local-windows-with-goarch:
	CGO_ENABLED=0 GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)-windows-$(GOARCH)$(RACE).exe $(PKG_PREFIX)/app/$(APP_NAME)

quicktemplate-gen: install-qtc
	qtc

install-qtc:
	which qtc || GO111MODULE=off go get github.com/valyala/quicktemplate/qtc


golangci-lint: install-golangci-lint
	golangci-lint run --exclude '(SA4003|SA1019|SA5011):' -D errcheck -D structcheck --timeout 2m

install-golangci-lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.46.2

install-wwhrd:
	which wwhrd || GO111MODULE=off go get github.com/frapposelli/wwhrd

check-licenses: install-wwhrd
	wwhrd check -f .wwhrd.yml

copy-docs:
	echo "---\nsort: ${ORDER}\n---\n" > ${DST}
	cat ${SRC} >> ${DST}

# Copies docs for all components and adds the order tag.
# Cluster docs are supposed to be ordered as 9th.
# For The rest of docs is ordered manually.t
docs-sync:
	cp README.md docs/README.md
	SRC=README.md DST=docs/Single-server-VictoriaMetrics.md ORDER=1 $(MAKE) copy-docs
	SRC=app/vmagent/README.md DST=docs/vmagent.md ORDER=3 $(MAKE) copy-docs
	SRC=app/vmalert/README.md DST=docs/vmalert.md ORDER=4 $(MAKE) copy-docs
	SRC=app/vmauth/README.md DST=docs/vmauth.md ORDER=5 $(MAKE) copy-docs
	SRC=app/vmbackup/README.md DST=docs/vmbackup.md ORDER=6 $(MAKE) copy-docs
	SRC=app/vmrestore/README.md DST=docs/vmrestore.md ORDER=7 $(MAKE) copy-docs
	SRC=app/vmctl/README.md DST=docs/vmctl.md ORDER=8 $(MAKE) copy-docs
	SRC=app/vmgateway/README.md DST=docs/vmgateway.md ORDER=9 $(MAKE) copy-docs
	SRC=app/vmbackupmanager/README.md DST=docs/vmbackupmanager.md ORDER=10 $(MAKE) copy-docs

# temporary operation to maintain compatibility
copy-linux:
	cd bin && \
		cp victoria-metrics-linux-amd64-$(PKG_TAG).tar.gz victoria-metrics-amd64-$(PKG_TAG).tar.gz && \
		cp victoria-metrics-linux-arm-$(PKG_TAG).tar.gz victoria-metrics-arm-$(PKG_TAG).tar.gz && \
		cp victoria-metrics-linux-arm64-$(PKG_TAG).tar.gz victoria-metrics-arm64-$(PKG_TAG).tar.gz && \
		cp vmutils-linux-amd64-$(PKG_TAG).tar.gz vmutils-amd64-$(PKG_TAG).tar.gz && \
		cp vmutils-linux-arm-$(PKG_TAG).tar.gz vmutils-arm-$(PKG_TAG).tar.gz && \
		cp vmutils-linux-arm64-$(PKG_TAG).tar.gz vmutils-arm64-$(PKG_TAG).tar.gz

copy-linux-cluster:
	cd bin && \
		cp victoria-metrics-linux-amd64-$(PKG_TAG)-cluster.tar.gz victoria-metrics-amd64-$(PKG_TAG)-cluster.tar.gz && \
		cp victoria-metrics-linux-arm-$(PKG_TAG)-cluster.tar.gz victoria-metrics-arm-$(PKG_TAG)-cluster.tar.gz && \
		cp victoria-metrics-linux-arm64-$(PKG_TAG)-cluster.tar.gz victoria-metrics-arm64-$(PKG_TAG)-cluster.tar.gz
		cp vmutils-linux-amd64-$(PKG_TAG)-cluster.tar.gz vmutils-amd64-$(PKG_TAG)-cluster.tar.gz && \
		cp vmutils-linux-arm-$(PKG_TAG)-cluster.tar.gz vmutils-arm-$(PKG_TAG)-cluster.tar.gz && \
		cp vmutils-linux-arm64-$(PKG_TAG)-cluster.tar.gz vmutils-arm64-$(PKG_TAG)-cluster.tar.gz

copy-linux-enterprise:
	cd bin && \
		cp victoria-metrics-linux-amd64-$(PKG_TAG)-enterprise.tar.gz victoria-metrics-amd64-$(PKG_TAG)-enterprise.tar.gz && \
		cp victoria-metrics-linux-arm-$(PKG_TAG)-enterprise.tar.gz victoria-metrics-arm-$(PKG_TAG)-enterprise.tar.gz && \
		cp victoria-metrics-linux-arm64-$(PKG_TAG)-enterprise.tar.gz victoria-metrics-arm64-$(PKG_TAG)-enterprise.tar.gz && \
		cp vmutils-linux-amd64-$(PKG_TAG)-enterprise.tar.gz vmutils-amd64-$(PKG_TAG)-enterprise.tar.gz && \
		cp vmutils-linux-arm-$(PKG_TAG)-enterprise.tar.gz vmutils-arm-$(PKG_TAG)-enterprise.tar.gz && \
		cp vmutils-linux-arm64-$(PKG_TAG)-enterprise.tar.gz vmutils-arm64-$(PKG_TAG)-enterprise.tar.gz

copy-linux-enterprise-cluster:
	cd bin && \
		cp victoria-metrics-linux-amd64-$(PKG_TAG)-enterprise-cluster.tar.gz victoria-metrics-amd64-$(PKG_TAG)-enterprise-cluster.tar.gz && \
		cp victoria-metrics-linux-arm-$(PKG_TAG)-enterprise-cluster.tar.gz victoria-metrics-arm-$(PKG_TAG)-enterprise-cluster.tar.gz && \
		cp victoria-metrics-linux-arm64-$(PKG_TAG)-enterprise-cluster.tar.gz victoria-metrics-arm64-$(PKG_TAG)-enterprise-cluster.tar.gz && \
		cp vmutils-linux-amd64-$(PKG_TAG)-enterprise-cluster.tar.gz vmutils-amd64-$(PKG_TAG)-enterprise-cluster.tar.gz && \
		cp vmutils-linux-arm-$(PKG_TAG)-enterprise-cluster.tar.gz vmutils-arm-$(PKG_TAG)-enterprise-cluster.tar.gz && \
		cp vmutils-linux-arm64-$(PKG_TAG)-enterprise-cluster.tar.gz vmutils-arm64-$(PKG_TAG)-enterprise-cluster.tar.gz
