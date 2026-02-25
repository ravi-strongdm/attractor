BIN      := attractor
BUILD_DIR := bin
CMD      := ./cmd/attractor

# Detect OS for path separator.
GOPATH_BIN := $(shell go env GOPATH)/bin

.PHONY: all build test lint vet tidy clean install examples help

all: tidy build test lint ## Run everything (default target)

build: ## Build the attractor binary
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BIN) $(CMD)

test: ## Run all tests with race detector
	go test -race ./...

test-verbose: ## Run tests with verbose output
	go test -race -v ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Run go mod tidy
	go mod tidy

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

install: build ## Install binary to GOPATH/bin
	cp $(BUILD_DIR)/$(BIN) $(GOPATH_BIN)/$(BIN)

examples: build ## Lint all example pipelines
	@for f in examples/*.dot; do \
		echo "  lint $$f"; \
		$(BUILD_DIR)/$(BIN) lint $$f || exit 1; \
	done
	@echo "All example pipelines are valid."

help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
