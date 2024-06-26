GO_VERSION ?= 1.22.4
SNAP_BUILDER_IMAGE := local/snap-builder:2.0.0-$(shell echo $(GO_VERSION) | tr :/ __)
TARGET_ARCH ?= amd64 arm arm64 ppc64le i386
TARGET_APPS ?= victoria-metrics

# truncate pkg_tag, snap has limitations for name length
SNAP_TAG := $(shell echo "$(PKG_TAG)" | cut -b1-31 )

snap-builder-image:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(SNAP_BUILDER_IMAGE)$$') \
	|| docker build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--tag $(SNAP_BUILDER_IMAGE) \
		snap/local

build-snap: snap-builder-image
	$(foreach app,$(TARGET_APPS), \
		$(foreach arch,$(TARGET_ARCH), \
			docker run --rm \
				--mount type=bind,src="${PWD}",dst=/builder \
				-w /builder \
				--env PKG_TAG=$(SNAP_TAG) \
				--env APP_NAME=$(app) \
				$(SNAP_BUILDER_IMAGE) \
				bash -c '\
					snapcraft clean && \
					snapcraft snap \
						-o "bin/$(app)_$(SNAP_TAG)_$(arch).snap" \
						--target-arch $(arch)'; \
		) \
	)

release-snap: snap-builder-image build-snap
	$(foreach app,$(TARGET_APPS), \
		$(foreach arch,$(TARGET_ARCH), \
			docker run --rm \
				--mount type=bind,src=${HOME}/.snap,dst=/root/.snap \
				--mount type=bind,src="${PWD}",dst=/builder \
				-w /builder \
				--env PKG_TAG=$(SNAP_TAG) \
				--env APP_NANE=$(app) \
				$(SNAP_BUILDER_IMAGE) \
				bash -c '\
					snapcraft login \
						--with /root/.snap/login.json && \
					snapcraft upload "bin/$(app)_$(SNAP_TAG)_$(arch).snap" \
						--release beta,edge,candidate,stable'; \
		) \
	)

