.PHONY: build test lint generate install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" ./cmd/ani-br

test:
	go test ./... -race

generate:
	go generate ./...

lint:
	golangci-lint run ./...

install:
	go install ./cmd/ani-br

