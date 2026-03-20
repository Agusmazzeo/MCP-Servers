.PHONY: help build-zencrm build-allfunds build-all clean test tidy install-deps

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-deps: ## Install dependencies for all modules
	@echo "Installing dependencies..."
	cd shared && go mod download
	cd services/zencrm && go mod download
	cd services/allfunds && go mod download
	@echo "✓ Dependencies installed"

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	cd shared && go mod tidy
	cd services/zencrm && go mod tidy
	cd services/allfunds && go mod tidy
	@echo "✓ Modules tidied"

build-zencrm: ## Build ZenCRM MCP server
	@echo "Building ZenCRM MCP server..."
	cd services/zencrm && go build -o ../../bin/zencrm-mcp ./cmd
	@echo "✓ Built: bin/zencrm-mcp"

build-allfunds: ## Build Allfunds MCP server
	@echo "Building Allfunds MCP server..."
	cd services/allfunds && go build -o ../../bin/allfunds-mcp ./cmd
	@echo "✓ Built: bin/allfunds-mcp"

build-all: build-zencrm build-allfunds ## Build all services
	@echo "✓ All services built"

run-zencrm: ## Run ZenCRM MCP server in stdio mode
	@cd services/zencrm && go run ./cmd -mode=stdio

test: ## Run tests for all modules
	@echo "Testing shared package..."
	cd shared && go test ./...
	@echo "Testing ZenCRM service..."
	cd services/zencrm && go test ./...
	@echo "✓ All tests passed"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	@echo "✓ Cleaned"

.DEFAULT_GOAL := help
