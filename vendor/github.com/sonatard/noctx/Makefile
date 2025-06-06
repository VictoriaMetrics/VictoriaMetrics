.PHONY: all imports test lint build

all: imports test lint build

imports:
	goimports -w ./

test:
	go test -race ./...

test_coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

lint:
	golangci-lint run ./...

build:
	go build -ldflags "-s -w" -trimpath ./cmd/noctx/
