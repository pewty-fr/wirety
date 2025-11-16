.PHONY: help build run test clean deps swagger build-all build-platform docker-build docker-run lint fmt install-tools

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	go mod download
	go mod tidy

GO ?= go
BINARY := wirety-server
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DIST_DIR := dist

build: ## Build the application for host
	$(GO) build -o $(DIST_DIR)/$(BINARY) ./cmd/server

build-platform: ## Build one platform (OS, ARCH) e.g. make build-platform OS=linux ARCH=arm64
	@[ -n "$(OS)" ] || (echo 'OS required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	@[ -n "$(ARCH)" ] || (echo 'ARCH required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY)-$(OS)-$(ARCH) ./cmd/server

build-all: ## Cross-compile server for preset platforms
	@mkdir -p $(DIST_DIR)
	@for plat in $(PLATFORMS); do \
		OS=$${plat%/*}; ARCH=$${plat#*/}; \
		 echo "Building $$OS/$$ARCH"; \
		 CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH $(GO) build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY)-$$OS-$$ARCH ./cmd/server || exit 1; \
	done

run: ## Run the application
	go run ./cmd/main.go

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/ $(DIST_DIR)
	rm -f coverage.out coverage.html

swagger: ## Generate Swagger documentation
	swag init -g cmd/main.go -o docs/swagger

lint: ## Run linters
	golangci-lint run

fmt: ## Format code
	go fmt ./...

docker-build: ## Build Docker image
	docker build -t wirety-server:latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 wirety-server:latest

install-tools: ## Install development tools
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.DEFAULT_GOAL := help
