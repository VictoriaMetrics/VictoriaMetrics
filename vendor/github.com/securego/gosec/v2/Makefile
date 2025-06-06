GIT_TAG?= $(shell git describe --always --tags)
BIN = gosec
FMT_CMD = $(gofmt -s -l -w $(find . -type f -name '*.go' -not -path './vendor/*') | tee /dev/stderr)
IMAGE_REPO = securego
DATE_FMT=+%Y-%m-%d
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE ?= $(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE ?= $(shell date "$(DATE_FMT)")
endif
BUILDFLAGS := "-w -s -X 'main.Version=$(GIT_TAG)' -X 'main.GitTag=$(GIT_TAG)' -X 'main.BuildDate=$(BUILD_DATE)'"
CGO_ENABLED = 0
GO := GO111MODULE=on go
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin
GOSEC ?= $(GOBIN)/gosec
GINKGO ?= $(GOBIN)/ginkgo
GO_MINOR_VERSION = $(shell $(GO) version | cut -c 14- | cut -d' ' -f1 | cut -d'.' -f2)
GOVULN_MIN_VERSION = 17
GO_VERSION = 1.23

default:
	$(MAKE) build

install-test-deps:
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	go install golang.org/x/crypto/...@latest
	go install github.com/lib/pq/...@latest

install-govulncheck:
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi

test: install-test-deps build-race fmt vet sec govulncheck
	$(GINKGO) -v --fail-fast

fmt:
	@echo "FORMATTING"
	@FORMATTED=`$(GO) fmt ./...`
	@([ ! -z "$(FORMATTED)" ] && printf "Fixed unformatted files:\n$(FORMATTED)") || true

vet:
	@echo "VETTING"
	$(GO) vet ./...

golangci:
	@echo "LINTING: golangci-lint"
	golangci-lint run

sec:
	@echo "SECURITY SCANNING"
	./$(BIN) ./...

govulncheck: install-govulncheck
	@echo "CHECKING VULNERABILITIES"
	@if [ $(GO_MINOR_VERSION) -gt $(GOVULN_MIN_VERSION) ]; then \
		govulncheck ./...; \
	fi

test-coverage: install-test-deps
	go test -race -v -count=1 -coverprofile=coverage.out ./...

build:
	go build -o $(BIN) ./cmd/gosec/

build-race:
	go build -race -o $(BIN) ./cmd/gosec/

clean:
	rm -rf build vendor dist coverage.out
	rm -f release image $(BIN)

release:
	@echo "Releasing the gosec binary..."
	goreleaser release

build-linux:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux go build -ldflags=$(BUILDFLAGS) -o $(BIN) ./cmd/gosec/

image:
	@echo "Building the Docker image..."
	docker build -t $(IMAGE_REPO)/$(BIN):$(GIT_TAG) --build-arg GO_VERSION=$(GO_VERSION) .
	docker tag $(IMAGE_REPO)/$(BIN):$(GIT_TAG) $(IMAGE_REPO)/$(BIN):latest
	touch image

image-push: image
	@echo "Pushing the Docker image..."
	docker push $(IMAGE_REPO)/$(BIN):$(GIT_TAG)
	docker push $(IMAGE_REPO)/$(BIN):latest

tlsconfig:
	go generate ./...

perf-diff:
	./perf-diff.sh

.PHONY: test build clean release image image-push tlsconfig perf-diff
