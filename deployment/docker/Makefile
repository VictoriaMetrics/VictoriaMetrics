# All these commands must run from repository root.

DOCKER_REGISTRY ?= docker.io
DOCKER_NAMESPACE ?= victoriametrics

ROOT_IMAGE ?= alpine:3.20.1
ROOT_IMAGE_SCRATCH ?= scratch
SKIP_SCRATCH_BUILD ?= false
CERTS_IMAGE := alpine:3.20.1

GO_BUILDER_IMAGE := golang:1.22.5-alpine
BUILDER_IMAGE := local/builder:2.0.0-$(shell echo $(GO_BUILDER_IMAGE) | tr :/ __)-1
BASE_IMAGE := local/base:1.1.4-$(shell echo $(ROOT_IMAGE) | tr :/ __)-$(shell echo $(CERTS_IMAGE) | tr :/ __)
DOCKER ?= docker
DOCKER_BUILD_OPTS ?=
DOCKER_RUN ?= $(DOCKER) run
DOCKER_BUILD ?= $(DOCKER) build
DOCKER_COMPOSE ?= $(DOCKER) compose
DOCKER_IMAGE_LS ?= $(DOCKER) image ls --format '{{.Repository}}:{{.Tag}}'
TARGET_PLATFORM ?= linux/amd64,linux/arm,linux/arm64,linux/ppc64le,linux/386
COMMA := ,

package-base:
	($(DOCKER_IMAGE_LS) | grep -q '$(BASE_IMAGE)$$') \
		|| $(DOCKER_BUILD) \
			--build-arg root_image=$(ROOT_IMAGE) \
			--build-arg certs_image=$(CERTS_IMAGE) \
			--tag $(BASE_IMAGE) \
			$(DOCKER_BUILD_OPTS) \
			deployment/docker/base

package-builder:
	($(DOCKER_IMAGE_LS) | grep -q '$(BUILDER_IMAGE)$$') \
		|| $(DOCKER_BUILD) \
			--build-arg go_builder_image=$(GO_BUILDER_IMAGE) \
			--tag $(BUILDER_IMAGE) \
			$(DOCKER_BUILD_OPTS) \
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
			-tags 'netgo osusergo nethttpomithttp2 musl' \
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
			-tags 'netgo osusergo nethttpomithttp2' \
			-o bin/$(APP_NAME)-windows$(APP_SUFFIX)-prod.exe $(PKG_PREFIX)/app/$(APP_NAME)

package-via-docker: package-base
	($(DOCKER_IMAGE_LS) | grep -q '$(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE)$$') || (\
		$(MAKE) app-via-docker && \
		$(DOCKER_BUILD) \
			--build-arg src_binary=$(APP_NAME)$(APP_SUFFIX)-prod \
			--build-arg base_image=$(BASE_IMAGE) \
			--tag $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) \
			-f app/$(APP_NAME)/deployment/Dockerfile bin)

publish-via-docker:
	$(eval TARGET_PLATFORMS := $(subst $(COMMA), ,$(TARGET_PLATFORM)))
	$(MAKE_PARALLEL) $(foreach PLATFORM,$(TARGET_PLATFORMS),app-via-docker-$(subst /,-,$(PLATFORM)))
	$(DOCKER) buildx build \
		--platform=$(TARGET_PLATFORM) \
		--build-arg certs_image=$(CERTS_IMAGE) \
		--build-arg root_image=$(ROOT_IMAGE) \
		--build-arg APP_NAME=$(APP_NAME) \
		--tag $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) \
		--tag $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(LATEST_TAG)$(RACE) \
		-o type=image \
		--provenance=false \
		$(DOCKER_BUILD_OPTS) \
		-f app/$(APP_NAME)/multiarch/Dockerfile \
		--push \
		bin
	$(if $(findstring $(SKIP_SCRATCH_BUILD),true),, \
		$(DOCKER) buildx build \
			--platform=$(TARGET_PLATFORM) \
			--build-arg certs_image=$(CERTS_IMAGE) \
			--build-arg root_image=$(ROOT_IMAGE_SCRATCH) \
			--build-arg APP_NAME=$(APP_NAME) \
			--tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)-scratch \
			--tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(LATEST_TAG)$(RACE)-scratch \
			-o type=image \
			--provenance=false \
			$(DOCKER_BUILD_OPTS) \
			-f app/$(APP_NAME)/multiarch/Dockerfile \
			--push \
			bin)
	cd bin && rm -rf $(foreach PLATFORM,$(TARGET_PLATFORMS),$(APP_NAME)-$(subst /,-,$(PLATFORM))-prod)

run-via-docker: package-via-docker
	$(DOCKER_RUN) -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_OPTS) \
		$(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) $(ARGS)

app-via-docker-goos-goarch:
	APP_SUFFIX='-$(GOOS)-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=$(GOOS) --env GOARCH=$(GOARCH) $(foreach v,$(EXTRA_DOCKER_ENVS),--env $(v))' \
	$(MAKE) app-via-docker

app-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

app-via-docker-linux-amd64:
	EXTRA_DOCKER_ENVS='CC=/opt/cross-builder/x86_64-linux-musl-cross/bin/x86_64-linux-musl-gcc' \
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-arm64:
	EXTRA_DOCKER_ENVS='CC=/opt/cross-builder/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc' \
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-arm:
	EXTRA_DOCKER_ENVS='GOARM=5' \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-ppc64le:
	CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le $(MAKE) app-via-docker-goos-goarch

app-via-docker-linux-386:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 $(MAKE) app-via-docker-goos-goarch

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
	EXTRA_DOCKER_ENVS='CC=/opt/cross-builder/x86_64-linux-musl-cross/bin/x86_64-linux-musl-gcc' \
	CGO_ENABLED=1 GOARCH=amd64 $(MAKE) package-via-docker-goarch

package-via-docker-arm64:
	EXTRA_DOCKER_ENVS='CC=/opt/cross-builder/aarch64-linux-musl-cross/bin/aarch64-linux-musl-gcc' \
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

docker-single-up:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose.yml up -d

docker-single-down:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose.yml down -v

docker-single-vm-datasource-up:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose.yml -f deployment/docker/vm-datasource/docker-compose.yml up -d

docker-single-vm-datasource-down:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose.yml -f deployment/docker/vm-datasource/docker-compose.yml down -v

docker-cluster-up:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-cluster.yml up -d

docker-cluster-down:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-cluster.yml down -v

docker-cluster-vm-datasource-up:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-cluster.yml -f deployment/docker/vm-datasource/docker-compose-cluster.yml up -d

docker-cluster-vm-datasource-down:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-cluster.yml -f deployment/docker/vm-datasource/docker-compose-cluster.yml down -v

docker-victorialogs-up:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-victorialogs.yml up -d

docker-victorialogs-down:
	$(DOCKER_COMPOSE) -f deployment/docker/docker-compose-victorialogs.yml down -v
