.PHONY: help build run test clean deps swagger

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o bin/wirety-server ./cmd/server

run: ## Run the application
	go run ./cmd/main.go

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/
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
