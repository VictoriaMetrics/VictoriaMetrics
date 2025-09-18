GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOOS_GOARCH := $(GOOS)_$(GOARCH)
GOOS_GOARCH_NATIVE := $(shell go env GOHOSTOS)_$(shell go env GOHOSTARCH)
LIBZSTD_NAME := libzstd_$(GOOS_GOARCH).a
ZSTD_VERSION ?= v1.5.7
ZIG_VERSION ?= 0.15.1
BASE_IMAGE ?= alpine:latest
BUILDER_IMAGE := local/builder:2.0.0-$(shell echo $(BASE_IMAGE) | tr : _ | tr / _)-zig-$(ZIG_VERSION)
.PHONY: libzstd.a $(LIBZSTD_NAME)

libzstd.a: $(LIBZSTD_NAME)
$(LIBZSTD_NAME):
ifeq ($(GOOS_GOARCH),$(GOOS_GOARCH_NATIVE))
	rm -f $(LIBZSTD_NAME)
	cd zstd/lib && ZSTD_LEGACY_SUPPORT=0 MOREFLAGS=$(MOREFLAGS) $(MAKE) clean libzstd.a
	mv zstd/lib/libzstd.a $(LIBZSTD_NAME)
else ifeq ($(GOOS_GOARCH),linux_amd64)
	TARGET=x86_64-linux GOARCH=amd64 GOOS=linux $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),linux_arm)
	TARGET=arm-linux-gnueabi GOARCH=arm GOOS=linux $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),linux_arm64)
	TARGET=aarch64-linux GOARCH=arm64 GOOS=linux $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),linux_ppc64le)
	TARGET=x86_64-linux GOARCH=ppc64le GOOS=linux $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),linux_musl_amd64)
	TARGET=x86_64-linux-musl GOARCH=amd64 GOOS=linux_musl $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),linux_musl_arm64)
	TARGET=aarch64-linux-musl GOARCH=arm64 GOOS=linux_musl $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),darwin_arm64)
	TARGET=aarch64-macos GOARCH=arm64 GOOS=darwin $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),darwin_amd64)
	TARGET=x86_64-macos GOARCH=amd64 GOOS=darwin $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),windows_amd64)
	TARGET=x86_64-windows GOARCH=amd64 GOOS=windows $(MAKE) package-arch
else ifeq ($(GOOS_GOARCH),freebsd_amd64)
	TARGET=x86_64-freebsd GOARCH=amd64 GOOS=freebsd $(MAKE) package-arch
endif

package-builder:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(BUILDER_IMAGE)$$') \
		|| docker build \
			--build-arg builder_image=$(BASE_IMAGE) \
			--build-arg zig_version=$(ZIG_VERSION) \
			--tag $(BUILDER_IMAGE) \
			builder

package-arch: package-builder
	rm -f $(LIBZSTD_NAME)
	docker run --rm \
		--mount type=bind,src="$(shell pwd)",dst=/zstd \
		-w /zstd \
		$(DOCKER_OPTS) \
		$(BUILDER_IMAGE) \
		-c 'cd zstd/lib && \
			ZSTD_LEGACY_SUPPORT=0 AR="zig ar" \
			CC="zig cc -target $(TARGET)" \
			CXX="zig cc -target $(TARGET)" \
			MOREFLAGS=$(MOREFLAGS) \
			RM="rm -rf --" \
			make clean libzstd.a'
	mv -f zstd/lib/libzstd.a $(LIBZSTD_NAME)

# illumos isn't supported by zig compiler atm.
release:
	GOOS=linux GOARCH=amd64 $(MAKE) libzstd.a
	GOOS=linux GOARCH=arm64 $(MAKE) libzstd.a
	GOOS=linux GOARCH=arm $(MAKE) libzstd.a
	GOOS=linux GOARCH=ppc64le $(MAKE) libzstd.a
	GOOS=linux_musl GOARCH=amd64 $(MAKE) libzstd.a
	GOOS=linux_musl GOARCH=arm64 $(MAKE) libzstd.a
	GOOS=darwin GOARCH=arm64 $(MAKE) libzstd.a
	GOOS=darwin GOARCH=amd64 $(MAKE) libzstd.a
	GOOS=windows GOARCH=amd64 $(MAKE) libzstd.a
	GOOS=freebsd GOARCH=amd64 $(MAKE) libzstd.a

clean:
	rm -f $(LIBZSTD_NAME)
	cd zstd && $(MAKE) clean

update-zstd:
	rm -rf zstd-tmp
	git clone --branch $(ZSTD_VERSION) --depth 1 https://github.com/Facebook/zstd zstd-tmp
	rm -rf zstd-tmp/.git
	rm -rf zstd
	mv zstd-tmp zstd
	cp zstd/lib/zstd.h .
	cp zstd/lib/zdict.h .
	cp zstd/lib/zstd_errors.h .
	$(MAKE) release

test:
	CGO_ENABLED=1 GOEXPERIMENT=cgocheck2 go test -v

bench:
	CGO_ENABLED=1 go test -bench=.
