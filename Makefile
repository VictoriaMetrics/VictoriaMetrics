PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics

BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | sha1sum | grep -oP '^.{8}')))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(shell date -u +'%Y%m%d-%H%M%S')-$(BUILDINFO_TAG)'

all: \
	victoria-metrics-prod

include app/*/Makefile
include deployment/*/Makefile

clean:
	rm -rf bin/*

publish: publish-victoria-metrics

package: package-victoria-metrics

release: victoria-metrics-prod
	cd bin && tar czf victoria-metrics-$(PKG_TAG).tar.gz victoria-metrics-prod

fmt:
	gofmt -l -w -s ./lib
	gofmt -l -w -s ./app

vet:
	go vet $(PKG_PREFIX)/lib/...
	go vet $(PKG_PREFIX)/app/...

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
	go test $(PKG_PREFIX)/lib/...

benchmark:
	go test -bench=. $(PKG_PREFIX)/lib/...

vendor-update:
	go get -u
	go mod tidy
	go mod vendor

quicktemplate-gen: install-qtc
	qtc

install-qtc:
	which qtc || GO111MODULE=off go get -u github.com/valyala/quicktemplate/qtc
