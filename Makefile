BOLD=$(shell tput bold)
RED=$(shell tput setaf 1)
BLUE=$(shell tput setaf 4)
RESET=$(shell tput sgr0)

PKG_PREFIX := github.com/VictoriaMetrics/VictoriaMetrics
ENTERPRISE_REMOTE_EXISTS=$(shell grep -Fx '[remote "enterprise"]' .git/config > /dev/null && echo 1 || echo 0)

DATEINFO_TAG ?= $(shell date -u +'%Y%m%d-%H%M%S')
BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -c 10-17)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

GO_BUILDINFO = -X '$(PKG_PREFIX)/lib/buildinfo.Version=$(APP_NAME)-$(DATEINFO_TAG)-$(BUILDINFO_TAG)'

.PHONY: $(MAKECMDGOALS)

include app/*/Makefile
include deployment/*/Makefile
include snap/local/Makefile

all: \
	victoria-metrics-prod \
	vmagent-prod \
	vmalert-prod \
	vmauth-prod \
	vmbackup-prod \
	vmrestore-prod \
	vmctl-prod

clean:
	rm -rf bin/*

publish: docker-scan \
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

vmutils-linux-amd64: \
	vmagent-linux-amd64 \
	vmalert-linux-amd64 \
	vmauth-linux-amd64 \
	vmbackup-linux-amd64 \
	vmrestore-linux-amd64 \
	vmctl-linux-amd64

vmutils-linux-arm64: \
	vmagent-linux-arm64 \
	vmalert-linux-arm64 \
	vmauth-linux-arm64 \
	vmbackup-linux-arm64 \
	vmrestore-linux-arm64 \
	vmctl-linux-arm64

vmutils-linux-arm: \
	vmagent-linux-arm \
	vmalert-linux-arm \
	vmauth-linux-arm \
	vmbackup-linux-arm \
	vmrestore-linux-arm \
	vmctl-linux-arm

vmutils-linux-386: \
	vmagent-linux-386 \
	vmalert-linux-386 \
	vmauth-linux-386 \
	vmbackup-linux-386 \
	vmrestore-linux-386 \
	vmctl-linux-386

vmutils-linux-ppc64le: \
	vmagent-linux-ppc64le \
	vmalert-linux-ppc64le \
	vmauth-linux-ppc64le \
	vmbackup-linux-ppc64le \
	vmrestore-linux-ppc64le \
	vmctl-linux-ppc64le

vmutils-darwin-amd64: \
	vmagent-darwin-amd64 \
	vmalert-darwin-amd64 \
	vmauth-darwin-amd64 \
	vmbackup-darwin-amd64 \
	vmrestore-darwin-amd64 \
	vmctl-darwin-amd64

vmutils-darwin-arm64: \
	vmagent-darwin-arm64 \
	vmalert-darwin-arm64 \
	vmauth-darwin-arm64 \
	vmbackup-darwin-arm64 \
	vmrestore-darwin-arm64 \
	vmctl-darwin-arm64

vmutils-freebsd-amd64: \
	vmagent-freebsd-amd64 \
	vmalert-freebsd-amd64 \
	vmauth-freebsd-amd64 \
	vmbackup-freebsd-amd64 \
	vmrestore-freebsd-amd64 \
	vmctl-freebsd-amd64

vmutils-openbsd-amd64: \
	vmagent-openbsd-amd64 \
	vmalert-openbsd-amd64 \
	vmauth-openbsd-amd64 \
	vmbackup-openbsd-amd64 \
	vmrestore-openbsd-amd64 \
	vmctl-openbsd-amd64

vmutils-windows-amd64: \
	vmagent-windows-amd64 \
	vmalert-windows-amd64 \
	vmauth-windows-amd64 \
	vmctl-windows-amd64

victoria-metrics-crossbuild: \
	victoria-metrics-linux-amd64 \
	victoria-metrics-linux-arm64 \
	victoria-metrics-linux-arm \
	victoria-metrics-linux-386 \
	victoria-metrics-linux-ppc64le \
	victoria-metrics-darwin-amd64 \
	victoria-metrics-darwin-arm64 \
	victoria-metrics-freebsd-amd64 \
	victoria-metrics-openbsd-amd64

vmutils-crossbuild: \
	vmutils-linux-amd64 \
	vmutils-linux-arm64 \
	vmutils-linux-arm \
	vmutils-linux-386 \
	vmutils-linux-ppc64le \
	vmutils-darwin-amd64 \
	vmutils-darwin-arm64 \
	vmutils-freebsd-amd64 \
	vmutils-openbsd-amd64 \
	vmutils-windows-amd64

publish-release:
	git checkout $(TAG) && $(MAKE) release publish && \
		git checkout $(TAG)-cluster && $(MAKE) release publish && \
		git checkout $(TAG)-enterprise && $(MAKE) release publish && \
		git checkout $(TAG)-enterprise-cluster && $(MAKE) release publish && \
		$(MAKE) github-create-release

release: \
	release-victoria-metrics \
	release-vmutils

release-victoria-metrics: \
	release-victoria-metrics-linux-amd64 \
	release-victoria-metrics-linux-arm \
	release-victoria-metrics-linux-arm64 \
	release-victoria-metrics-darwin-amd64 \
	release-victoria-metrics-darwin-arm64 \
	release-victoria-metrics-freebsd-amd64 \
	release-victoria-metrics-openbsd-amd64

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

release-victoria-metrics-goos-goarch: victoria-metrics-$(GOOS)-$(GOARCH)-prod
	cd bin && \
		tar --transform="flags=r;s|-$(GOOS)-$(GOARCH)||" -czf victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(GOOS)-$(GOARCH)-prod \
		&& sha256sum victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			victoria-metrics-$(GOOS)-$(GOARCH)-prod \
			| sed s/-$(GOOS)-$(GOARCH)-prod/-prod/ > victoria-metrics-$(GOOS)-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf victoria-metrics-$(GOOS)-$(GOARCH)-prod

release-vmutils: \
	release-vmutils-linux-amd64 \
	release-vmutils-linux-arm64 \
	release-vmutils-linux-arm \
	release-vmutils-darwin-amd64 \
	release-vmutils-darwin-arm64 \
	release-vmutils-freebsd-amd64 \
	release-vmutils-openbsd-amd64 \
	release-vmutils-windows-amd64

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
	vmauth-$(GOOS)-$(GOARCH)-prod \
	vmbackup-$(GOOS)-$(GOARCH)-prod \
	vmrestore-$(GOOS)-$(GOARCH)-prod \
	vmctl-$(GOOS)-$(GOARCH)-prod
	cd bin && \
		tar --transform="flags=r;s|-$(GOOS)-$(GOARCH)||" -czf vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(GOOS)-$(GOARCH)-prod \
			vmalert-$(GOOS)-$(GOARCH)-prod \
			vmauth-$(GOOS)-$(GOARCH)-prod \
			vmbackup-$(GOOS)-$(GOARCH)-prod \
			vmrestore-$(GOOS)-$(GOARCH)-prod \
			vmctl-$(GOOS)-$(GOARCH)-prod \
		&& sha256sum vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG).tar.gz \
			vmagent-$(GOOS)-$(GOARCH)-prod \
			vmalert-$(GOOS)-$(GOARCH)-prod \
			vmauth-$(GOOS)-$(GOARCH)-prod \
			vmbackup-$(GOOS)-$(GOARCH)-prod \
			vmrestore-$(GOOS)-$(GOARCH)-prod \
			vmctl-$(GOOS)-$(GOARCH)-prod \
			| sed s/-$(GOOS)-$(GOARCH)-prod/-prod/ > vmutils-$(GOOS)-$(GOARCH)-$(PKG_TAG)_checksums.txt
	cd bin && rm -rf \
		vmagent-$(GOOS)-$(GOARCH)-prod \
		vmalert-$(GOOS)-$(GOARCH)-prod \
		vmauth-$(GOOS)-$(GOARCH)-prod \
		vmbackup-$(GOOS)-$(GOARCH)-prod \
		vmrestore-$(GOOS)-$(GOARCH)-prod \
		vmctl-$(GOOS)-$(GOARCH)-prod

release-vmutils-windows-goarch: \
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
	cd bin && rm -rf \
		vmagent-windows-$(GOARCH)-prod.exe \
		vmalert-windows-$(GOARCH)-prod.exe \
		vmauth-windows-$(GOARCH)-prod.exe \
		vmctl-windows-$(GOARCH)-prod.exe


pprof-cpu:
	go tool pprof -trim_path=github.com/VictoriaMetrics/VictoriaMetrics@ $(PPROF_FILE)

fmt:
	gofmt -l -w -s ./lib
	gofmt -l -w -s ./app

vet:
	go vet ./lib/...
	go vet ./app/...

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
	go test ./lib/... ./app/...

test-race:
	go test -race ./lib/... ./app/...

test-pure:
	CGO_ENABLED=0 go test ./lib/... ./app/...

test-full:
	go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

test-full-386:
	GOARCH=386 go test -coverprofile=coverage.txt -covermode=atomic ./lib/... ./app/...

benchmark:
	go test -bench=. ./lib/...
	go test -bench=. ./app/...

benchmark-pure:
	CGO_ENABLED=0 go test -bench=. ./lib/...
	CGO_ENABLED=0 go test -bench=. ./app/...

vendor-update:
	go get -u -d ./lib/...
	go get -u -d ./app/...
	go mod tidy -compat=1.18
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
	which qtc || GO111MODULE=off go get github.com/valyala/quicktemplate/qtc


golangci-lint: install-golangci-lint
	golangci-lint run --exclude '(SA4003|SA1019|SA5011):' -D errcheck -D structcheck --timeout 2m

install-golangci-lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.48.0

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

enterprise-remote-check:
ifeq ($(ENTERPRISE_REMOTE_EXISTS), 0)
	$(error enterprise remote is missing)
endif

create-tags: enterprise-remote-check
	git fetch --all
	git checkout -b enterprise enterprise/enterprise && git tag -s $(TAG)-enterprise -m "$(TAG)"
	git checkout -b enterprise-cluster enterprise/enterprise-cluster && git tag -s $(TAG)-enterprise-cluster -m "$(TAG)"
	git checkout -b cluster origin/cluster && git tag -s $(TAG)-cluster -m "$(TAG)"
	git checkout -b master origin/master && git tag -s $(TAG) -m "$(TAG)"

push-tags: enterprise-remote-check
	git push origin $(TAG)
	git push origin $(TAG)-cluster
	git push enterprise $(TAG)-enterprise
	git push enterprise $(TAG)-enterprise-cluster

github-token-check:
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif

github-create-release: github-token-check push-tags
	@release_id=$$(curl -s -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token ${GITHUB_TOKEN}" \
		https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases \
		-d '{"tag_name":"$(TAG)","name":"$(TAG)","body":"$(TAG)","draft":true,"prerelease":false,"generate_release_notes":false}' | grep '"id"' -m 1 | sed -E 's/.* ([[:digit:]]+)\,/\1/' ) && \
	printf "Created release for ${BOLD}${BLUE}${TAG}${RESET}\n"; \
	$(foreach file, $(wildcard bin/*.zip), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/zip" $(MAKE) github-upload-asset;) \
	$(foreach file, $(wildcard bin/*.tar.gz), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/x-gzip" $(MAKE) github-upload-asset;) \
	$(foreach file, $(wildcard bin/*_checksums.txt), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="text/plain" $(MAKE) github-upload-asset;) 

github-upload-asset:
	@printf "Uploading ${BOLD}$(FILE)${RESET}\n"
	@result=$$(curl -o /dev/null -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token ${GITHUB_TOKEN}" \
		-H "Content-Type: $(CONTENT_TYPE)" \
		--data-binary "@$(FILE)" \
		https://uploads.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases/$(RELEASE_ID)/assets?name=$(notdir $(FILE))) ; \
		if [ $${result} = 201 ]; then \
			printf "Upload OK: $${result}\n"; \
		else \
			printf "Upload ${RED}Failed${RESET}: $${result}\n"; \
			exit 1; \
		fi
