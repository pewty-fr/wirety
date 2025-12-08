GO ?= go
PKG := ./...
BINARY := wirety-agent
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
BUILDTIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
DIST_DIR := bin
REGISTRY ?= rg.fr-par.scw.cloud/wirety
IMAGE_NAME ?= agent
IMAGE_TAG ?= $(VERSION)

.PHONY: build run test lint docker-build docker-build-multiarch docker-push clean build-all build-platform help

help: ## Show help
	@echo 'Targets:'
	@echo '  build              Build for host OS/ARCH'
	@echo '  build-all          Build for $(PLATFORMS)'
	@echo '  build-platform     Build single platform: make build-platform OS=linux ARCH=arm64'
	@echo '  docker-build       Build docker image for current platform'
	@echo '  docker-build-multiarch Build multi-architecture docker images'
	@echo '  docker-push        Push docker images to registry'
	@echo '  test               Run tests'
	@echo '  lint               Vet packages'
	@echo '  clean              Remove build artifacts'

build: ## Build host
	$(GO) build -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILDTIME) -X main.commit=$(COMMIT)" -o $(DIST_DIR)/$(BINARY) ./cmd/agent

run: build ## Run host build
	./$(DIST_DIR)/$(BINARY)

build-platform: ## Build one platform (OS, ARCH)
	@[ -n "$(OS)" ] || (echo 'OS required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	@[ -n "$(ARCH)" ] || (echo 'ARCH required e.g. make build-platform OS=linux ARCH=arm64' && exit 1)
	mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILDTIME) -X main.commit=$(COMMIT)" -o $(DIST_DIR)/$(BINARY)-$(OS)-$(ARCH) ./cmd/agent

build-all: ## Build all platforms
	@mkdir -p $(DIST_DIR)
	@for plat in $(PLATFORMS); do \
		OS=$${plat%/*}; ARCH=$${plat#*/}; \
		 echo "Building $$OS/$$ARCH"; \
		 CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH $(GO) build -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILDTIME) -X main.commit=$(COMMIT)" -o $(DIST_DIR)/$(BINARY)-$$OS-$$ARCH ./cmd/agent || exit 1; \
	done

docker-build: ## Build docker image for current platform
	docker build -t $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILDTIME=$(BUILDTIME) \
		--build-arg COMMIT=$(COMMIT) \
		.
	docker tag $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):latest

docker-build-multiarch: ## Build multi-architecture docker images
	docker buildx create --name wirety-builder --use --bootstrap || true
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILDTIME=$(BUILDTIME) \
		--build-arg COMMIT=$(COMMIT) \
		-t $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(REGISTRY)/$(IMAGE_NAME):latest \
		-f Dockerfile.multiarch \
		--push \
		.

docker-push: ## Push docker images to registry
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):latest

test: ## Run tests
	$(GO) test -count=1 $(PKG)

lint: ## Vet packages
	$(GO) vet $(PKG)
	golangci-lint run || true

clean: ## Clean artifacts
	rm -rf $(DIST_DIR)
	docker buildx rm wirety-builder || true
