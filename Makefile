.PHONY: help build-zencrm build-allfunds build-1816 build-oms build-all clean test tidy install-deps refresh-deps

SERVERS := zencrm allfunds 1816 oms

help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-deps: ## Install dependencies for all modules
	@echo "Installing dependencies..."
	@for server in $(SERVERS); do \
		echo "  → servers/$$server"; \
		(cd servers/$$server && go mod download); \
	done
	@echo "✓ Dependencies installed"

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	@for server in $(SERVERS); do \
		echo "  → servers/$$server"; \
		(cd servers/$$server && go mod tidy); \
	done
	@echo "✓ Modules tidied"

refresh-deps: ## Refresh all dependencies to latest versions
	@echo "Refreshing dependencies..."
	@for server in $(SERVERS); do \
		echo "  → servers/$$server"; \
		(cd servers/$$server && go get -u ./... && go mod tidy); \
	done
	@echo "✓ Dependencies refreshed"

build-zencrm: ## Build ZenCRM MCP server
	@echo "Building ZenCRM MCP server..."
	cd servers/zencrm && go build -o ../../bin/zencrm-mcp ./cmd
	@echo "✓ Built: bin/zencrm-mcp"

build-allfunds: ## Build Allfunds MCP server
	@echo "Building Allfunds MCP server..."
	cd servers/allfunds && go build -o ../../bin/allfunds-mcp ./cmd
	@echo "✓ Built: bin/allfunds-mcp"

build-1816: ## Build 1816 MCP server
	@echo "Building 1816 MCP server..."
	cd servers/1816 && go build -o ../../bin/1816-mcp ./cmd
	@echo "✓ Built: bin/1816-mcp"

build-oms: ## Build OMS MCP server
	@echo "Building OMS MCP server..."
	cd servers/oms && go build -o ../../bin/oms-mcp ./cmd
	@echo "✓ Built: bin/oms-mcp"

build-all: build-zencrm build-allfunds build-1816 build-oms ## Build all services
	@echo "✓ All services built"

run-zencrm: ## Run ZenCRM MCP server in stdio mode
	@cd servers/zencrm && go run ./cmd -mode=stdio

run-allfunds: ## Run Allfunds MCP server in stdio mode
	@cd servers/allfunds && go run ./cmd -mode=stdio

run-1816: ## Run 1816 MCP server in stdio mode
	@cd servers/1816 && go run ./cmd -mode=stdio

run-oms: ## Run OMS MCP server in stdio mode
	@cd servers/oms && go run ./cmd -mode=stdio

test: ## Run tests for all modules
	@echo "Running tests..."
	@for server in $(SERVERS); do \
		echo "  → servers/$$server"; \
		(cd servers/$$server && go test ./...); \
	done
	@echo "✓ All tests passed"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	@echo "✓ Cleaned"

.DEFAULT_GOAL := help
