# All these commands must run from repository root.

DOCKER_REGISTRIES ?= docker.io quay.io
DOCKER_NAMESPACE ?= victoriametrics

ROOT_IMAGE ?= alpine:3.23.3
ROOT_IMAGE_SCRATCH ?= scratch
CERTS_IMAGE := alpine:3.23.3

GO_BUILDER_IMAGE := golang:1.26.0

# SBOM generation settings
SYFT_VERSION ?= 1.26.1
ORAS_VERSION ?= 1.2.2
SBOM_FORMAT ?= cyclonedx-json
SBOM_MEDIA_TYPE ?= application/vnd.cyclonedx+json

# Host platform detection for checksum lookup
HOST_OS   := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

# SHA256 checksums for syft and oras binaries (update when changing versions)
SYFT_SHA256_darwin_amd64 := 2eae0b76a208c5916cf02847b94e861024c7a5a6c1e2e606f5436f97747b1f76
SYFT_SHA256_darwin_arm64 := 00435a3fe2ae940203708ee2eae9976d1719982c628d30b2b78aacd36133ec6b
SYFT_SHA256_linux_amd64  := 4f3e84f9467080c876deb0fa968da54309c6d21fb8c00fd3a4e547eb9f006835
SYFT_SHA256_linux_arm64  := ed3915cbc9c039f0501cb49d4485125befbd729acc263e767f70a18de3fec10d
ORAS_SHA256_darwin_amd64 := e953c3c5580a317ba30871ee77f39a37e5bcb030edf01c5e9a83ebddcec7cf8a
ORAS_SHA256_darwin_arm64 := fcac22adaf1cfe7854738859d0f4705d1cd75a23edaef5ee2e9aba66cfff2315
ORAS_SHA256_linux_amd64  := bff970346470e5ef888e9f2c0bf7f8ee47283f5a45207d6e7a037da1fb0eae0d
ORAS_SHA256_linux_arm64  := edd7195cbb8ba56c29ede413eefa10c8026201d63326017cd315841b4063aa56

SYFT_SHA256 := $(SYFT_SHA256_$(HOST_OS)_$(HOST_ARCH))
ORAS_SHA256 := $(ORAS_SHA256_$(HOST_OS)_$(HOST_ARCH))

BUILDER_IMAGE := local/builder:2.0.0-$(shell echo $(GO_BUILDER_IMAGE) | tr :/ __)-1
BASE_IMAGE := local/base:1.1.4-$(shell echo $(ROOT_IMAGE) | tr :/ __)-$(shell echo $(CERTS_IMAGE) | tr :/ __)
DOCKER ?= docker
DOCKER_RUN ?= $(DOCKER) run
DOCKER_BUILD ?= $(DOCKER) build
DOCKER_COMPOSE ?= $(DOCKER) compose
DOCKER_IMAGE_LS ?= $(DOCKER) image ls --format '{{.Repository}}:{{.Tag}}'

package-base:
	($(DOCKER_IMAGE_LS) | grep -q '$(BASE_IMAGE)$$') \
		|| $(DOCKER_BUILD) \
			--build-arg root_image=$(ROOT_IMAGE) \
			--build-arg certs_image=$(CERTS_IMAGE) \
			--tag $(BASE_IMAGE) \
			deployment/docker/base

package-builder:
	($(DOCKER_IMAGE_LS) | grep -q '$(BUILDER_IMAGE)$$') \
		|| $(DOCKER_BUILD) \
			--build-arg go_builder_image=$(GO_BUILDER_IMAGE) \
			--tag $(BUILDER_IMAGE) \
			deployment/docker/builder

app-via-docker: package-builder
	mkdir -p gocache-for-docker
	$(DOCKER_RUN) --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)",dst=/VictoriaMetrics \
		-w /VictoriaMetrics \
		--mount type=bind,src="$(shell pwd)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		$(DOCKER_OPTS) \
		$(BUILDER_IMAGE) \
		go build $(RACE) -trimpath -buildvcs=false \
			-ldflags "-extldflags '-static' $(GO_BUILDINFO)" \
			-tags 'netgo osusergo $(EXTRA_GO_BUILD_TAGS)' \
			-o bin/$(APP_NAME)$(APP_SUFFIX)-prod $(PKG_PREFIX)/app/$(APP_NAME)

app-via-docker-windows: package-builder
	mkdir -p gocache-for-docker
	$(DOCKER_RUN) --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)",dst=/VictoriaMetrics \
		-w /VictoriaMetrics \
		--mount type=bind,src="$(shell pwd)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		$(DOCKER_OPTS) \
		$(BUILDER_IMAGE) \
		go build $(RACE) -trimpath -buildvcs=false \
			-ldflags "-s -w -extldflags '-static' $(GO_BUILDINFO)" \
			-tags 'netgo osusergo $(EXTRA_GO_BUILD_TAGS)' \
			-o bin/$(APP_NAME)-windows$(APP_SUFFIX)-prod.exe $(PKG_PREFIX)/app/$(APP_NAME)

package-via-docker: package-base
	($(DOCKER_IMAGE_LS) | grep -q '$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE)$$') || (\
		$(MAKE) app-via-docker && \
		$(DOCKER_BUILD) \
			--build-arg src_binary=$(APP_NAME)$(APP_SUFFIX)-prod \
			--build-arg base_image=$(BASE_IMAGE) \
			--label "org.opencontainers.image.source=https://github.com/VictoriaMetrics/VictoriaMetrics" \
			--label "org.opencontainers.image.documentation=https://docs.victoriametrics.com/" \
			--label "org.opencontainers.image.title=$(APP_NAME)" \
			--label "org.opencontainers.image.vendor=VictoriaMetrics" \
			--label "org.opencontainers.image.version=$(PKG_TAG)" \
			--label "org.opencontainers.image.created=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")" \
			--tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) \
			-f app/$(APP_NAME)/deployment/Dockerfile bin)

publish-via-docker:
	$(MAKE_PARALLEL) app-via-docker-linux-amd64 \
		app-via-docker-linux-arm \
		app-via-docker-linux-arm64 \
		app-via-docker-linux-ppc64le \
		app-via-docker-linux-386
	$(DOCKER) buildx build \
		--platform=linux/amd64,linux/arm,linux/arm64,linux/ppc64le,linux/386 \
		--build-arg certs_image=$(CERTS_IMAGE) \
		--build-arg root_image=$(ROOT_IMAGE) \
		--build-arg APP_NAME=$(APP_NAME) \
		--build-arg BINARY_SUFFIX="" \
		--label "org.opencontainers.image.source=https://github.com/VictoriaMetrics/VictoriaMetrics" \
		--label "org.opencontainers.image.documentation=https://docs.victoriametrics.com/" \
		--label "org.opencontainers.image.title=$(APP_NAME)" \
		--label "org.opencontainers.image.vendor=VictoriaMetrics" \
		--label "org.opencontainers.image.version=$(PKG_TAG)" \
		--label "org.opencontainers.image.created=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		$(foreach registry,$(DOCKER_REGISTRIES),\
		--tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)$(EXTRA_DOCKER_TAG_SUFFIX) \
		) \
		-o type=image \
		--provenance=false \
		-f app/$(APP_NAME)/multiarch/Dockerfile \
		--push \
		bin
	$(DOCKER) buildx build \
		--platform=linux/amd64,linux/arm,linux/arm64,linux/ppc64le,linux/386 \
		--build-arg certs_image=$(CERTS_IMAGE) \
		--build-arg root_image=$(ROOT_IMAGE_SCRATCH) \
		--build-arg APP_NAME=$(APP_NAME) \
		--build-arg BINARY_SUFFIX="" \
		--label "org.opencontainers.image.source=https://github.com/VictoriaMetrics/VictoriaMetrics" \
		--label "org.opencontainers.image.documentation=https://docs.victoriametrics.com/" \
		--label "org.opencontainers.image.title=$(APP_NAME)" \
		--label "org.opencontainers.image.vendor=VictoriaMetrics" \
		--label "org.opencontainers.image.version=$(PKG_TAG)" \
		--label "org.opencontainers.image.created=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")" \
		$(foreach registry,$(DOCKER_REGISTRIES),\
		--tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)$(EXTRA_DOCKER_TAG_SUFFIX)-scratch \
		) \
		-o type=image \
		--provenance=false \
		-f app/$(APP_NAME)/multiarch/Dockerfile \
		--push \
		bin
	-$(MAKE) publish-sbom
	cd bin && rm -rf \
		$(APP_NAME)-linux-amd64-prod \
		$(APP_NAME)-linux-arm-prod \
		$(APP_NAME)-linux-arm64-prod \
		$(APP_NAME)-linux-ppc64le-prod \
		$(APP_NAME)-linux-386-prod

publish-via-docker-from-rc:
	$(foreach registry,$(DOCKER_REGISTRIES),\
		docker buildx imagetools create --tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG) $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(EXTRA_DOCKER_TAG_SUFFIX); \
	)
	$(foreach registry,$(DOCKER_REGISTRIES),\
		docker buildx imagetools create --tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-scratch $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(EXTRA_DOCKER_TAG_SUFFIX)-scratch; \
	)
	@if echo "$(PKG_TAG)" | grep -q "enterprise"; then \
		$(foreach registry,$(DOCKER_REGISTRIES),\
			docker buildx imagetools create --tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-fips $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(EXTRA_DOCKER_TAG_SUFFIX)-fips; \
		) \
	fi
	-$(MAKE) publish-sbom-from-rc

publish-via-docker-latest:
	$(foreach registry,$(DOCKER_REGISTRIES),\
		docker buildx imagetools create --tag $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):latest $(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG); \
	)
	-$(MAKE) publish-sbom-latest

install-syft:
	@which syft >/dev/null 2>&1 || { \
		echo "Installing syft $(SYFT_VERSION)..."; \
		if [ -z "$(SYFT_SHA256)" ]; then echo "ERROR: No syft checksum for $(HOST_OS)_$(HOST_ARCH)"; exit 1; fi && \
		SYFT_TMP=$$(mktemp -d) && \
		curl -sSfL "https://github.com/anchore/syft/releases/download/v$(SYFT_VERSION)/syft_$(SYFT_VERSION)_$(HOST_OS)_$(HOST_ARCH).tar.gz" \
			-o "$$SYFT_TMP/syft.tar.gz" && \
		echo "$(SYFT_SHA256)  $$SYFT_TMP/syft.tar.gz" | sha256sum -c - && \
		tar -xzf "$$SYFT_TMP/syft.tar.gz" -C "$$SYFT_TMP" syft && \
		sudo install "$$SYFT_TMP/syft" /usr/local/bin/syft && \
		rm -rf "$$SYFT_TMP" && \
		echo "syft $(SYFT_VERSION) installed"; \
	}

install-oras:
	@which oras >/dev/null 2>&1 || { \
		echo "Installing oras $(ORAS_VERSION)..."; \
		if [ -z "$(ORAS_SHA256)" ]; then echo "ERROR: No oras checksum for $(HOST_OS)_$(HOST_ARCH)"; exit 1; fi && \
		ORAS_TMP=$$(mktemp -d) && \
		curl -sSfL "https://github.com/oras-project/oras/releases/download/v$(ORAS_VERSION)/oras_$(ORAS_VERSION)_$(HOST_OS)_$(HOST_ARCH).tar.gz" \
			-o "$$ORAS_TMP/oras.tar.gz" && \
		echo "$(ORAS_SHA256)  $$ORAS_TMP/oras.tar.gz" | sha256sum -c - && \
		tar -xzf "$$ORAS_TMP/oras.tar.gz" -C "$$ORAS_TMP" oras && \
		sudo install "$$ORAS_TMP/oras" /usr/local/bin/oras && \
		rm -rf "$$ORAS_TMP" && \
		echo "oras $(ORAS_VERSION) installed"; \
	}

# generate-and-attach-sbom: Generate SBOM for a single image and attach it via ORAS.
# Required vars: SBOM_IMAGE (full image ref, e.g. docker.io/victoriametrics/vmagent:v1.100.0)
generate-and-attach-sbom: install-syft install-oras
	@SBOM_TMPDIR=$$(mktemp -d) && \
	trap 'rm -rf "$$SBOM_TMPDIR"' EXIT && \
	SBOM_FILE="sbom.cdx.json" && \
	echo "Generating SBOM for $(SBOM_IMAGE)..." && \
	syft "$(SBOM_IMAGE)" -o "$(SBOM_FORMAT)"="$$SBOM_TMPDIR/$$SBOM_FILE" && \
	echo "Validating SBOM..." && \
	jq -e '.bomFormat' "$$SBOM_TMPDIR/$$SBOM_FILE" >/dev/null 2>&1 && \
	echo "Attaching SBOM to $(SBOM_IMAGE)..." && \
	cd "$$SBOM_TMPDIR" && \
	oras attach --artifact-type "$(SBOM_MEDIA_TYPE)" \
		--distribution-spec v1.1-referrers-tag \
		"$(SBOM_IMAGE)" \
		"$$SBOM_FILE":"$(SBOM_MEDIA_TYPE)" && \
	echo "SBOM attached successfully to $(SBOM_IMAGE)" || \
	echo "WARNING: SBOM generation/attachment failed for $(SBOM_IMAGE)"

publish-sbom:
	$(foreach registry,$(DOCKER_REGISTRIES),\
		SBOM_IMAGE=$(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)$(EXTRA_DOCKER_TAG_SUFFIX) $(MAKE) generate-and-attach-sbom || true; \
		SBOM_IMAGE=$(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)$(EXTRA_DOCKER_TAG_SUFFIX)-scratch $(MAKE) generate-and-attach-sbom || true; \
	)

publish-sbom-from-rc:
	$(foreach registry,$(DOCKER_REGISTRIES),\
		SBOM_IMAGE=$(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG) $(MAKE) generate-and-attach-sbom || true; \
		SBOM_IMAGE=$(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-scratch $(MAKE) generate-and-attach-sbom || true; \
	)

publish-sbom-latest:
	$(foreach registry,$(DOCKER_REGISTRIES),\
		SBOM_IMAGE=$(registry)/$(DOCKER_NAMESPACE)/$(APP_NAME):latest $(MAKE) generate-and-attach-sbom || true; \
	)

verify-sbom: install-oras
	@echo "Verifying SBOM for $(SBOM_IMAGE)..."
	@oras discover --artifact-type "$(SBOM_MEDIA_TYPE)" "$(SBOM_IMAGE)" | grep -q "$(SBOM_MEDIA_TYPE)" \
		&& echo "SBOM found for $(SBOM_IMAGE)" \
		|| (echo "ERROR: No SBOM found for $(SBOM_IMAGE)" && exit 1)

run-via-docker: package-via-docker
	$(DOCKER_RUN) -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_OPTS) \
		$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) $(ARGS)

app-via-docker-goos-goarch:
	APP_SUFFIX='-$(GOOS)-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=$(GOOS) --env GOARCH=$(GOARCH) $(foreach v,$(EXTRA_DOCKER_ENVS),--env $(v))' \
	$(MAKE) app-via-docker

app-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

app-via-docker-linux-amd64:
	EXTRA_DOCKER_ENVS='CC=x86_64-linux-gnu-gcc' \
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-arm64:
	EXTRA_DOCKER_ENVS='CC=aarch64-linux-gnu-gcc' \
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-arm:
	EXTRA_DOCKER_ENVS='GOARM=5' \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-s390x:
	CGO_ENABLED=0 GOOS=linux GOARCH=s390x $(MAKE) app-via-docker-goos-goarch

app-via-docker-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-freebsd-amd64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-openbsd-amd64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-windows-amd64:
	APP_SUFFIX='-amd64' \
	DOCKER_OPTS='--env CGO_ENABLED=0 --env GOOS=windows --env GOARCH=amd64' \
	$(MAKE) app-via-docker-windows

package-via-docker-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=linux --env GOARCH=$(GOARCH) $(foreach v,$(EXTRA_DOCKER_ENVS),--env $(v))' \
	$(MAKE) package-via-docker

package-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) package-via-docker

package-via-docker-amd64:
	EXTRA_DOCKER_ENVS='CC=x86_64-linux-gnu-gcc' \
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) package-via-docker-goarch

package-via-docker-arm64:
	EXTRA_DOCKER_ENVS='CC=aarch64-linux-gnu-gcc' \
	CGO_ENABLED=1 GOARCH=arm64 $(MAKE) package-via-docker-goarch

package-via-docker-arm:
	EXTRA_DOCKER_ENVS='GOARM=5' \
	CGO_ENABLED=0 GOARCH=arm $(MAKE) package-via-docker-goarch

package-via-docker-ppc64le:
	CGO_ENABLED=0 GOARCH=ppc64le $(MAKE) package-via-docker-goarch

package-via-docker-386:
	CGO_ENABLED=0 GOARCH=386 $(MAKE) package-via-docker-goarch

remove-docker-images:
	docker image ls --format '{{.ID}}' | xargs docker image rm -f

# VM single
docker-vm-single-up:
	$(DOCKER_COMPOSE) -f deployment/docker/compose-vm-single.yml up -d

docker-vm-single-down:
	$(DOCKER_COMPOSE) -f deployment/docker/compose-vm-single.yml down -v

# VM cluster
docker-vm-cluster-up:
	$(DOCKER_COMPOSE) -f deployment/docker/compose-vm-cluster.yml up -d

docker-vm-cluster-down:
	$(DOCKER_COMPOSE) -f deployment/docker/compose-vm-cluster.yml down -v

# Command aliases to keep backward-compatibility, as they could have been mentioned on the Internet before the rename.
docker-single-up: docker-vm-single-up
docker-single-down: docker-vm-single-down

docker-cluster-up: docker-vm-cluster-up
docker-cluster-down: docker-vm-cluster-down
