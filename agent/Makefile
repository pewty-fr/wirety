GO ?= go
PKG := ./...
BINARY := wirety-agent
VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DIST_DIR := bin

.PHONY: build run test lint docker-build clean build-all build-platform help

help: ## Show help
	@echo 'Targets:'
	@echo '  build           Build for host OS/ARCH'
	@echo '  build-all       Build for $(PLATFORMS)'
	@echo '  build-platform  Build single platform: make build-platform OS=linux ARCH=arm64'
	@echo '  docker-build    Build docker image'
	@echo '  test            Run tests'
	@echo '  lint            Vet packages'
	@echo '  clean           Remove build artifacts'

build: ## Build host
	$(GO) build -o $(DIST_DIR)/$(BINARY) ./cmd/agent

run: build ## Run host build
	./$(DIST_DIR)/$(BINARY)

build-platform: ## Build one platform (OS, ARCH)
	@[ -n "$(OS)" ] || (echo 'OS required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	@[ -n "$(ARCH)" ] || (echo 'ARCH required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY)-$(OS)-$(ARCH) ./cmd/agent

build-all: ## Build all platforms
	@mkdir -p $(DIST_DIR)
	@for plat in $(PLATFORMS); do \
		OS=$${plat%/*}; ARCH=$${plat#*/}; \
		 echo "Building $$OS/$$ARCH"; \
		 CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH $(GO) build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY)-$$OS-$$ARCH ./cmd/agent || exit 1; \
	done

docker-build: build ## Build docker image
	docker build -t wirety-agent:latest .

test: ## Run tests
	$(GO) test -count=1 $(PKG)

lint: ## Vet packages
	$(GO) vet $(PKG)

clean: ## Clean artifacts
	rm -rf $(DIST_DIR)
