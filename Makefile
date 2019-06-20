PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics

BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | sha1sum | grep -oP '^.{8}')))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(shell date -u +'%Y%m%d-%H%M%S')-$(BUILDINFO_TAG)'

all: \
	vminsert \
	vmselect \
	vmstorage

include app/*/Makefile
include deployment/*/Makefile
include deployment/*/helm/Makefile

clean:
	rm -rf bin/*

publish: \
	publish-vmstorage \
	publish-vmselect \
	publish-vminsert

package: \
	package-vmstorage \
	package-vmselect \
	package-vminsert

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
	which golint || GO111MODULE=off go get -u github.com/golang/lint/golint

errcheck: install-errcheck
	errcheck -exclude=errcheck_excludes.txt ./lib/...
	errcheck -exclude=errcheck_excludes.txt ./app/vminsert/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmselect/...
	errcheck -exclude=errcheck_excludes.txt ./app/vmstorage/...

install-errcheck:
	which errcheck || GO111MODULE=off go get -u github.com/kisielk/errcheck

test:
	GO111MODULE=on go test -mod=vendor ./lib/...
	GO111MODULE=on go test -mod=vendor ./app/...

benchmark:
	GO111MODULE=on go test -mod=vendor -bench=. ./lib/...
	GO111MODULE=on go test -mod=vendor -bench=. ./app/...

vendor-update:
	GO111MODULE=on go get -u ./lib/...
	GO111MODULE=on go get -u ./app/...
	GO111MODULE=on go mod tidy
	GO111MODULE=on go mod vendor

app-local:
	GO111MODULE=on go build $(RACE) -mod=vendor -ldflags "$(GO_BUILDINFO)" -o bin/$(APP_NAME)$(RACE) $(PKG_PREFIX)/app/$(APP_NAME)

quicktemplate-gen: install-qtc
	qtc

install-qtc:
	which qtc || GO111MODULE=off go get -u github.com/valyala/quicktemplate/qtc


golangci-lint: install-golangci-lint
	golangci-lint run

install-golangci-lint:
	which golangci-lint || GO111MODULE=off go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
