# hlgo Makefile
# Default target chains fmt → vet → build so you can never build unformatted code.

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.version=$(VERSION)
BIN_DIR  := bin
DIST_DIR := dist

.PHONY: build test test-cover vet fmt lint tidy check clean install \
        build-linux build-darwin build-windows dist

## build (default): fmt → vet → compile
build: fmt vet
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/hlgo .

## test: run all tests with race detector
test:
	go test -race -count=1 ./...

## test-cover: test with coverage report
test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## vet: static analysis
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	gofmt -s -w .
	@command -v goimports >/dev/null 2>&1 && goimports -w . || true

## lint: run golangci-lint (not a build prereq — too slow for hot iteration)
lint:
	golangci-lint run

## tidy: tidy and verify module
tidy:
	go mod tidy
	go mod verify

## check: full pre-commit pipeline
check: fmt tidy vet lint test

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

## install: install with version injection
install:
	go install -ldflags "$(LDFLAGS)" .

# --- Cross-compilation ---

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/hlgo-linux-amd64 .

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/hlgo-darwin-arm64 .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/hlgo-windows-amd64.exe .

## dist: build for all platforms
dist: build-linux build-darwin build-windows
