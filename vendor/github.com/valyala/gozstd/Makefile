GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOOS_GOARCH := $(GOOS)_$(GOARCH)
GOOS_GOARCH_NATIVE := $(shell go env GOHOSTOS)_$(shell go env GOHOSTARCH)
LIBZSTD_NAME := libzstd_$(GOOS_GOARCH).a
ZSTD_VERSION ?= master

.PHONY: libzstd.a

libzstd.a: $(LIBZSTD_NAME)

$(LIBZSTD_NAME):
ifeq ($(GOOS_GOARCH),$(GOOS_GOARCH_NATIVE))
	cd zstd/lib && ZSTD_LEGACY_SUPPORT=0 MOREFLAGS=$(MOREFLAGS) $(MAKE) clean libzstd.a
	mv zstd/lib/libzstd.a $(LIBZSTD_NAME)
else
ifeq ($(GOOS_GOARCH),linux_arm)
	cd zstd/lib && CC=arm-linux-gnueabi-gcc ZSTD_LEGACY_SUPPORT=0 MOREFLAGS=$(MOREFLAGS) $(MAKE) clean libzstd.a
	mv zstd/lib/libzstd.a libzstd_linux_arm.a
endif
ifeq ($(GOOS_GOARCH),linux_arm64)
	cd zstd/lib && CC=aarch64-linux-gnu-gcc ZSTD_LEGACY_SUPPORT=0 MOREFLAGS=$(MOREFLAGS) $(MAKE) clean libzstd.a
	mv zstd/lib/libzstd.a libzstd_linux_arm64.a
endif
endif

clean:
	rm -f $(LIBZSTD_NAME)
	cd zstd && $(MAKE) clean

update-zstd:
	rm -rf zstd-tmp
	git clone --branch $(ZSTD_VERSION) --depth 1 https://github.com/Facebook/zstd zstd-tmp
	rm -rf zstd-tmp/.git
	rm -rf zstd
	mv zstd-tmp zstd
	$(MAKE) clean libzstd.a
	cp zstd/lib/zstd.h .
	cp zstd/lib/dictBuilder/zdict.h .
	cp zstd/lib/common/zstd_errors.h .

test:
	CGO_ENABLED=1 GODEBUG=cgocheck=2 go test -v

bench:
	CGO_ENABLED=1 go test -bench=.
