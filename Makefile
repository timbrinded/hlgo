# hlgo Makefile
# Default target chains fmt → vet → build so you can never build unformatted code.
SHELL := /bin/bash

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BIN_DIR  := bin
DIST_DIR := dist
TEST_OUT := /tmp/hlgo-test.out

.PHONY: build test test\:ci test-cover test-integration vet fmt lint tidy check clean install \
        build-linux build-darwin build-windows dist

# run-tests-quiet: local-friendly test runner — shows only failures + summary.
# Usage: $(call run-tests-quiet,<extra go test flags>)
define run-tests-quiet
	@go test -race -count=1 $(1) ./... 2>&1 | tee $(TEST_OUT); \
	status=$${PIPESTATUS[0]}; \
	echo ""; \
	echo "──────────────────────────────────────────"; \
	pass=$$(grep -c '^ok' $(TEST_OUT) || true); \
	fail=$$(grep -c '^FAIL' $(TEST_OUT) || true); \
	echo "Tests: $$pass passed, $$fail failed"; \
	echo "──────────────────────────────────────────"; \
	exit $$status
endef

# run-tests-verbose: CI-friendly — full verbose output + summary.
# Usage: $(call run-tests-verbose,<extra go test flags>)
define run-tests-verbose
	@go test -v -race -count=1 $(1) ./... 2>&1 | tee $(TEST_OUT); \
	status=$${PIPESTATUS[0]}; \
	echo ""; \
	echo "──────────────────────────────────────────"; \
	pass=$$(grep -c '^--- PASS' $(TEST_OUT) || true); \
	fail=$$(grep -c '^--- FAIL' $(TEST_OUT) || true); \
	echo "Tests: $$pass passed, $$fail failed"; \
	echo "──────────────────────────────────────────"; \
	exit $$status
endef

## build (default): fmt → vet → compile
build: fmt vet
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/hlgo .

## test: run all tests — quiet output, shows only failures + summary
test:
	$(call run-tests-quiet,)

## test:ci: run all tests — full verbose output for CI pipelines
test\:ci:
	$(call run-tests-verbose,)

## test-cover: test with coverage report
test-cover:
	$(call run-tests-quiet,-coverprofile=coverage.out)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-integration: run integration tests (requires testnet env vars)
test-integration:
	$(call run-tests-quiet,-tags=integration -timeout=5m)

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
