.PHONY: build run test clean install dev help

# Binary name
BINARY_NAME=pubsub-server

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ## Install dependencies
	$(GOGET) -v ./...
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the server binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/server
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: ## Run the server
	$(GORUN) ./cmd/server/main.go

dev: ## Run with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
	air

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Test coverage:"
	$(GOCMD) tool cover -func=coverage.out

bench: ## Run benchmarks
	$(GOTEST) -bench=. -benchmem ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

fmt: ## Format code
	$(GOCMD) fmt ./...
	$(GOCMD) vet ./...

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

docker-build: ## Build Docker image
	docker build -t pubsub-server:latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 pubsub-server:latest

# Create topics for testing
create-test-topics: ## Create test topics via REST API
	@echo "Creating test topics..."
	curl -X POST http://localhost:8080/topics -H "Content-Type: application/json" -d '{"name":"orders"}'
	curl -X POST http://localhost:8080/topics -H "Content-Type: application/json" -d '{"name":"notifications"}'
	curl -X POST http://localhost:8080/topics -H "Content-Type: application/json" -d '{"name":"events"}'
	@echo "\nTopics created!"

list-topics: ## List all topics
	@curl -s http://localhost:8080/topics | jq

stats: ## Show system statistics
	@curl -s http://localhost:8080/stats | jq

health: ## Check system health
	@curl -s http://localhost:8080/health | jq

all: clean install build ## Clean, install dependencies, and build
