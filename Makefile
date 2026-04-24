.PHONY: help build run clean deps tidy fmt vet test all

# Binary name
BINARY_NAME=cheryl-code
BINARY_PATH=./bin/$(BINARY_NAME)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Main package path
MAIN_PATH=./cmd

# Default target
all: deps fmt vet build

help: ## Display this help screen
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "Build complete: $(BINARY_PATH)"

run: ## Run the application (usage: make run PROMPT="your prompt here")
	@if [ -z "$(PROMPT)" ]; then \
		echo "Error: PROMPT is required. Usage: make run PROMPT=\"your prompt here\""; \
		exit 1; \
	fi
	$(GOCMD) run $(MAIN_PATH) -p "$(PROMPT)"

clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf bin/
	@echo "Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOGET) -v -d ./...
	@echo "Dependencies downloaded"

tidy: ## Tidy and verify dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify
	@echo "Dependencies tidied"

fmt: ## Format the code
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet complete"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

install: ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(MAIN_PATH)
	@echo "Installed to $(shell go env GOPATH)/bin/$(BINARY_NAME)"

dev: ## Run in development mode with auto-reload (requires air)
	@which air > /dev/null || (echo "air not found. Install it with: go install github.com/air-verse/air@latest" && exit 1)
	air

check: fmt vet test ## Run all checks (fmt, vet, test)

.DEFAULT_GOAL := help