SHELL := /bin/bash

default: build

build: ## Build and install the binary
	@echo "make: Building..."
	@go install

fmt: ## Format Go source code
	@echo "make: Formatting source code..."
	@find . -name '*.go' -exec gofmt -s -w {} +

fmt-check: ## Check Go source formatting
	@echo "make: Checking formatting..."
	@fmt_out=$$(find . -name '*.go' -exec gofmt -s -l {} +); \
	if [ -n "$$fmt_out" ]; then \
		echo "$$fmt_out"; \
		echo 'Code is not gofmt formatted. Run `make fmt`.'; \
		exit 1; \
	fi

help: ## Show this help message
	@awk 'BEGIN {FS = ":.*?## "}; /^[a-zA-Z0-9][^:]*:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: build ## Build and install the binary

lint: ## Run golangci-lint
	@golangci-lint run

modern: ## Fix code to use modern Go idioms
	@go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test ./...

modern-check: ## Check for modern Go idioms
	@go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -test ./...

staticcheck: ## Run staticcheck linter
	@staticcheck ./...

test: ## Run unit tests
	@echo "make: Running tests..."
	@go test ./... -timeout=5m

tidy: ## Run go mod tidy
	@go mod tidy

tidy-check: ## Check if go.mod and go.sum are tidy
	@cp go.mod go.mod.bak
	@cp go.sum go.sum.bak
	@go mod tidy
	@git diff --exit-code go.mod go.sum
	@mv go.mod.bak go.mod
	@mv go.sum.bak go.sum

vet: ## Run go vet
	@go vet ./...

vulncheck: ## Run govulncheck
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

.PHONY: build fmt fmt-check help install lint modern modern-check staticcheck test tidy tidy-check vet vulncheck
