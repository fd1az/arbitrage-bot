.PHONY: help build run clean test fmt vet tidy lint mocks install-tools dev

# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# Project variables
BINARY_NAME=arbitrage-bot
BUILD_DIR=bin
MAIN_PATH=./cmd/arbitrage
GO=go
GOFLAGS=-v

# Colors for terminal output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m
BOLD := \033[1m

help: ## Show this help message
	@echo "$(BOLD)$(BLUE)arbitrage-bot Makefile Commands$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""

# ============================================================================
# Build & Run
# ============================================================================

build: ## Build the arbitrage-bot binary
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

run: build ## Build and run the arbitrage-bot
	@echo "$(BLUE)Starting $(BINARY_NAME)...$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME)

run-tui: build ## Build and run in TUI mode
	@echo "$(BLUE)Starting $(BINARY_NAME) in TUI mode...$(NC)"
	@./$(BUILD_DIR)/$(BINARY_NAME) --tui

dev: ## Run in development mode with hot reload (requires air)
	@if ! command -v air > /dev/null; then \
		echo "$(YELLOW)Installing air for hot reload...$(NC)"; \
		go install github.com/cosmtrek/air@latest; \
	fi
	@air

clean: ## Remove build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@rm -rf mocks
	@echo "$(GREEN)Clean complete$(NC)"

# ============================================================================
# Testing
# ============================================================================

test: ## Run all tests
	@echo "$(BLUE)Running tests...$(NC)"
	@$(GO) test -v -race ./...
	@echo "$(GREEN)Tests complete$(NC)"

test-coverage: ## Run tests with coverage report
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@$(GO) test -v -race -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

test-short: ## Run short tests only
	@echo "$(BLUE)Running short tests...$(NC)"
	@$(GO) test -v -short ./...

test-integration: ## Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	@$(GO) test -v -tags=integration ./...

bench: ## Run benchmarks
	@echo "$(BLUE)Running benchmarks...$(NC)"
	@$(GO) test -bench=. -benchmem ./...

# ============================================================================
# Code Quality
# ============================================================================

fmt: ## Format Go code
	@echo "$(BLUE)Formatting code...$(NC)"
	@$(GO) fmt ./...
	@echo "$(GREEN)Format complete$(NC)"

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	@$(GO) vet ./...
	@echo "$(GREEN)Vet complete$(NC)"

lint: ## Run golangci-lint
	@if ! command -v golangci-lint > /dev/null; then \
		echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "$(BLUE)Running linter...$(NC)"
	@golangci-lint run ./...
	@echo "$(GREEN)Lint complete$(NC)"

check: fmt vet lint ## Run all code quality checks
	@echo "$(GREEN)All checks passed$(NC)"

# ============================================================================
# Mocks & Code Generation
# ============================================================================

mocks: ## Generate test mocks with mockery
	@if ! command -v mockery > /dev/null; then \
		echo "$(YELLOW)Installing mockery...$(NC)"; \
		go install github.com/vektra/mockery/v2@latest; \
	fi
	@echo "$(BLUE)Generating mocks...$(NC)"
	@rm -rf ./mocks
	@mockery --config .mockery.yaml
	@echo "$(GREEN)Mocks generated$(NC)"

generate: ## Run go generate
	@echo "$(BLUE)Running go generate...$(NC)"
	@$(GO) generate ./...
	@echo "$(GREEN)Generate complete$(NC)"

# ============================================================================
# Dependencies
# ============================================================================

deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	@$(GO) mod download
	@echo "$(GREEN)Dependencies downloaded$(NC)"

deps-update: ## Update dependencies
	@echo "$(BLUE)Updating dependencies...$(NC)"
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

tidy: ## Tidy Go modules
	@echo "$(BLUE)Tidying Go modules...$(NC)"
	@$(GO) mod tidy
	@echo "$(GREEN)Tidy complete$(NC)"

# ============================================================================
# Development Tools
# ============================================================================

install-tools: ## Install all development tools
	@echo "$(BLUE)Installing development tools...$(NC)"
	@go install github.com/cosmtrek/air@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/vektra/mockery/v2@latest
	@echo "$(GREEN)Tools installed$(NC)"

setup: install-tools deps ## Initial project setup
	@echo "$(BLUE)Setting up project...$(NC)"
	@if [ ! -f config.yaml ]; then \
		echo "$(YELLOW)Creating config.yaml from example...$(NC)"; \
		cp config.example.yaml config.yaml 2>/dev/null || echo "$(YELLOW)No config.example.yaml found$(NC)"; \
	fi
	@echo "$(GREEN)Project setup complete$(NC)"

# ============================================================================
# Docker
# ============================================================================

docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	@docker build -t $(BINARY_NAME):latest .
	@echo "$(GREEN)Docker image built$(NC)"

docker-run: docker-build ## Build and run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	@docker run --rm -it --env-file .env $(BINARY_NAME):latest

# ============================================================================
# Utility Commands
# ============================================================================

info: ## Show project information
	@echo "$(BOLD)$(BLUE)arbitrage-bot Project Information$(NC)"
	@echo ""
	@echo "  $(YELLOW)Binary:$(NC)      $(BINARY_NAME)"
	@echo "  $(YELLOW)Main Path:$(NC)   $(MAIN_PATH)"
	@echo "  $(YELLOW)Build Dir:$(NC)   $(BUILD_DIR)"
	@echo "  $(YELLOW)Go Version:$(NC)  $$($(GO) version | cut -d' ' -f3)"
	@echo ""
	@echo "$(BOLD)$(BLUE)Architecture:$(NC)"
	@echo "  - Hexagonal / Ports & Adapters"
	@echo "  - Bounded Contexts: pricing, blockchain, arbitrage"
	@echo ""
	@echo "$(BOLD)$(BLUE)Business Modules:$(NC)"
	@find business -type d -depth 1 2>/dev/null | sed 's/business\//  - /' | sort || echo "  (none)"
	@echo ""

list-modules: ## List all business modules
	@echo "$(BOLD)Business Modules:$(NC)"
	@find business -type d -depth 1 | sort

config-check: ## Validate configuration file
	@if [ ! -f config.yaml ]; then \
		echo "$(RED)Error: config.yaml not found$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)config.yaml exists$(NC)"

version: ## Show Go version
	@$(GO) version

all: clean check build test ## Clean, check, build, and test

# Default target
.DEFAULT_GOAL := help
